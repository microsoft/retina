// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

package capture

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
	common "github.com/microsoft/retina/test/integration/common"
)

const (
	podRunningCheckDuration = 2 * time.Minute
	podRunningCheckInterval = time.Second

	defaultDurationStr = "1m"
)

var (
	nodes []corev1.Node

	captureName string
	namespace   string

	unsetRetinaAgentEnv bool

	ictx *common.IntegrationContext
	l    *log.ZapLogger
)

func TestCapture(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Capture integration tests")
}

var _ = Describe("Capture flag tests", func() {
	BeforeSuite(func() {
		ictx = common.Init()
		l = ictx.Logger().Named("capture-test")
	})

	BeforeEach(func() {
		nodes = append(ictx.Nodes.Items, ictx.ControlPlaneNodes.Items...)
		Expect(len(nodes)).NotTo(Equal(0))

		// TODO(mainred): we currently build and load only mono-arch images to kind cluster, while by default Retina
		// capture requires multi-arch images when the debug mode is enabled.
		// As a workaround, we set RETINA_AGENT_IMAGE to mono-arch image for now when RETINA_AGENT_IMAGE env is not set
		// by user.
		if retinaAgentImagePreset := os.Getenv("RETINA_AGENT_IMAGE"); len(retinaAgentImagePreset) == 0 {
			retinaVersion := assertExecCommandOutput("kubectl", "retina", "version")
			retinaAgentImage := fmt.Sprintf("acnpublic.azurecr.io/retina-agent:%s", strings.TrimSpace(retinaVersion))
			os.Setenv("RETINA_AGENT_IMAGE", retinaAgentImage)
			unsetRetinaAgentEnv = true
		}
	})

	AfterEach(func() {
		if unsetRetinaAgentEnv {
			os.Unsetenv("RETINA_AGENT_IMAGE")
		}

		if CurrentGinkgoTestDescription().Failed {
			fetchAndPrintDebugInfo(namespace)
		}

		deleteCapture(captureName, namespace)
		deleteNamespace(namespace)
	})

	It("is running a capture only on one worker node (node-selector)", func() {
		// Create unique namespace for this test
		namespace = createNamespace("test-nodeselector")

		// Create capture on nodes with specific labels.
		nodeSelector := fmt.Sprintf("kubernetes.io/hostname=%s", ictx.Nodes.Items[0].Name)
		out := assertExecCommandOutput("kubectl", "retina", "capture", "create", fmt.Sprintf("--node-selectors=%s", nodeSelector), fmt.Sprintf("--namespace=%s", namespace), fmt.Sprintf("--duration=%s", defaultDurationStr), "--host-path=/mnt/test", "--debug")
		captureName = getCaptureName(out)

		// Wait for all capture jobs to start running a pod before checking for captures
		waitUntilCapturePodsStart(captureName, namespace)

		for _, node := range nodes {
			capturePodList, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{FieldSelector: "spec.nodeName=" + node.Name, LabelSelector: fmt.Sprintf("%s=%s", label.AppLabel, captureConstants.CaptureAppname)})
			Expect(err).ToNot(HaveOccurred())

			// Make sure there's only one capture on capture nodes, and none on non-capture nodes
			nodeSelectorMap, _ := labels.ConvertSelectorToLabelsMap(nodeSelector)
			nodeSelected := containsMap(nodeSelectorMap, node.Labels)
			expectedCaptureCount := 0
			if nodeSelected {
				expectedCaptureCount = 1
			}
			Expect(len(capturePodList.Items)).Should(Equal(expectedCaptureCount))
		}
	})

	It("capture should only be on nodes that have selected pods that are in selected namespaces (pod- and namespace-selector)", func() {
		// Create unique namespace for this test
		namespace = createNamespace("test-podselector")

		// Create pod
		jobName := "testjob"
		_ = assertExecCommandOutput("kubectl", "create", "job", jobName, "--image=hello-world", fmt.Sprintf("--namespace=%s", namespace))
		Eventually(func() bool {
			job, err := ictx.Clientset.BatchV1().Jobs(namespace).Get(context.TODO(), jobName, v1.GetOptions{})
			if err != nil {
				return false
			}
			return job.Status.Active == 1
		}, podRunningCheckDuration, podRunningCheckInterval).Should(BeTrue())

		// Create capture on nodes with selected pods in selected namespaces
		podSelector := "job-name=" + jobName
		namespaceSelector := "kubernetes.io/metadata.name=" + namespace
		out := assertExecCommandOutput("kubectl", "retina", "capture", "create", fmt.Sprintf("--pod-selectors=%s", podSelector), fmt.Sprintf("--namespace-selectors=%s", namespaceSelector), fmt.Sprintf("--namespace=%s", namespace), fmt.Sprintf("--duration=%s", defaultDurationStr), "--host-path=/mnt/test", "--debug")
		captureName = getCaptureName(out)

		// Wait for all capture jobs to start running a pod before checking for captures
		waitUntilCapturePodsStart(captureName, namespace)

		for _, node := range nodes {
			capturePodList, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{FieldSelector: "spec.nodeName=" + node.Name, LabelSelector: fmt.Sprintf("%s=%s", label.AppLabel, captureConstants.CaptureAppname)})
			Expect(err).ToNot(HaveOccurred())

			podList, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{FieldSelector: "spec.nodeName=" + node.Name})
			Expect(err).ToNot(HaveOccurred())

			podSelectorMap, _ := labels.ConvertSelectorToLabelsMap(podSelector)
			namespaceSelectorMap, _ := labels.ConvertSelectorToLabelsMap(namespaceSelector)
			selectedPods := 0
			for _, pod := range podList.Items {
				namespaceObj, err := ictx.Clientset.CoreV1().Namespaces().Get(context.TODO(), pod.Namespace, v1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				if containsMap(podSelectorMap, pod.Labels) && containsMap(namespaceSelectorMap, namespaceObj.Labels) {
					selectedPods++
				}
			}

			// Make sure there's only one capture on nodes that have selected pods, and no captures on nodes without selected pods
			Expect(len(capturePodList.Items)).Should(Equal(selectedPods))
		}
	})

	It("should verify the capture occurred in the specified namespace (namespace)", func() {
		// Create unique namespace for this test
		namespace = createNamespace("test-namespace")

		// Create capture on nodes with specific labels.
		nodeSelector := fmt.Sprintf("kubernetes.io/hostname=%s", ictx.Nodes.Items[0].Name)
		out := assertExecCommandOutput("kubectl", "retina", "capture", "create", fmt.Sprintf("--node-selectors=%s", nodeSelector), fmt.Sprintf("--namespace=%s", namespace), "--host-path=/mnt/test", "--debug")
		captureName = getCaptureName(out)

		// get capture from list in namespace
		out = assertExecCommandOutput("bash", "-c", fmt.Sprintf("kubectl retina capture list --namespace %s |grep -v NAMESPACE | awk '{print $2}'", namespace))
		capture := regexp.MustCompile("retina-capture-[a-zA-Z0-9]+").FindString(out)
		Expect(capture).NotTo(BeEmpty())
	})

	It("is running a capture for the specified duration (duration)", func() {
		// Create unique namespace for this test
		namespace = createNamespace("test-duration")

		testDuration := "30s"
		// Create capture on nodes with specific labels.
		nodeSelector := fmt.Sprintf("kubernetes.io/hostname=%s", ictx.Nodes.Items[0].Name)
		out := assertExecCommandOutput("bash", "-c", fmt.Sprintf("kubectl retina capture create --node-selectors=%s --namespace=%s --duration=%s --host-path=/mnt/test --debug", nodeSelector, namespace, testDuration))
		captureName = getCaptureName(out)

		// Wait for all capture jobs to start running a pod before checking for captures
		waitUntilCapturePodsStart(captureName, namespace)

		capturePods, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{LabelSelector: "capture-name=" + captureName})
		Expect(err).ToNot(HaveOccurred())
		Expect(capturePods.Items).NotTo(BeEmpty())
		captureContainerExists := false
		for _, container := range capturePods.Items[0].Spec.Containers {
			if container.Name == captureConstants.CaptureAppname {
				captureContainerExists = true
				for _, envVar := range container.Env {
					if envVar.Name == captureConstants.CaptureDurationEnvKey {
						Expect(strings.TrimSpace(envVar.Value)).To(Equal(testDuration))
					}
				}
			}
		}
		Expect(captureContainerExists).To(Equal(true))
	})

	It("should wait until the capture is completed when 'no-wait' is false (no-wait)", func() {
		// Create unique namespace for this test
		namespace = createNamespace("test-no-wait")

		// Create capture on nodes with specific labels.
		nodeSelector := fmt.Sprintf("kubernetes.io/hostname=%s", ictx.Nodes.Items[0].Name)
		cmd := exec.Command("kubectl", "retina", "capture", "create", fmt.Sprintf("--node-selectors=%s", nodeSelector), fmt.Sprintf("--namespace=%s", namespace), fmt.Sprintf("--duration=%s", defaultDurationStr), "--no-wait=false", "--host-path=/mnt/test", "--debug")
		err := cmd.Start() // Run command asynchronously
		Expect(err).ToNot(HaveOccurred())

		// Obtain capture name to delete after test.
		// Starting the command does not instantly create the capture, so we need to wait until it is created.
		var listCaptureOutput string
		Eventually(func() bool {
			listCaptureOutput = assertExecCommandOutput("bash", "-c", fmt.Sprintf("kubectl retina capture list --namespace %s |grep -v NAMESPACE | awk '{print $2}'", namespace))
			return strings.Contains(listCaptureOutput, "retina-capture")
		}, 10*time.Second, time.Second).Should(BeTrue())
		captureName = getCaptureName(listCaptureOutput)

		exited := cmd.ProcessState != nil && cmd.ProcessState.Exited()
		Expect(exited).To(Equal(false))

		err = cmd.Wait()
		Expect(err).ToNot(HaveOccurred())
		exited = cmd.ProcessState != nil && cmd.ProcessState.Exited()
		Expect(exited).To(Equal(true))
	})

	It("should have a max file size (max-size)", func() {
		fileSizeMB := 2
		hostPath := "/mnt/test"
		namespace = createNamespace("test-max-size")

		// Create capture with determined filesize
		nodeName := ictx.Nodes.Items[0].Name
		nodeSelector := fmt.Sprintf("kubernetes.io/hostname=%s", nodeName)
		// Give a big enough duration to not let duration decide when to stop the capture.
		cmd := exec.Command("bash", "-c", fmt.Sprintf("kubectl retina capture create --node-selectors=%s --namespace=%s --duration=10m --max-size=%d --host-path=%s --debug", nodeSelector, namespace, fileSizeMB, hostPath))
		err := cmd.Start()
		Expect(err).ToNot(HaveOccurred())

		// Obtain capture name to delete after test.
		// Starting the command does not instantly create the capture, so we need to wait until it is created.
		var listCaptureOutput string
		Eventually(func() bool {
			listCaptureOutput = assertExecCommandOutput("bash", "-c", fmt.Sprintf("kubectl retina capture list --namespace %s |grep -v NAMESPACE | awk '{print $2}'", namespace))
			return strings.Contains(listCaptureOutput, "retina-capture")
		}, 10*time.Second, time.Second).Should(BeTrue())
		captureName = getCaptureName(listCaptureOutput)

		capturePod := corev1.Pod{}
		// Make sure all pods are completed
		Eventually(func() bool {
			capturePods, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{LabelSelector: "capture-name=" + captureName})
			// In case there's no pods created
			if len(capturePods.Items) == 0 {
				return false
			}
			if err != nil {
				return false
			}
			for _, capturePod := range capturePods.Items {
				if capturePod.Status.Phase != corev1.PodSucceeded {
					return false
				}
			}
			capturePod = capturePods.Items[0]
			return true
		}, 10*time.Minute, podRunningCheckInterval).Should(BeTrue())

		// copy capture file to local
		err, targetFilePath := copyCaptureFileToLocal(capturePod)
		Expect(err).ToNot(HaveOccurred())

		// get size of pcap file and compare to expected size
		out := assertExecCommandOutput("bash", "-c", fmt.Sprintf("tar -ztvf  %s | grep pcap |  awk '{print $3}'", targetFilePath))
		sizeString := strings.Trim(out, "\n")
		Expect(sizeString).NotTo(BeEmpty())
		size, _ := strconv.Atoi(sizeString)
		// Small max file size may bring inaccuracy, while max big file size can be too slow to finish.
		// We allow the actual file size to be at most 2 times of the expected size when we pick a small max file size.
		Expect(size).To(BeNumerically(">", fileSizeMB*1000000/2))
		Expect(size).To(BeNumerically("<=", fileSizeMB*1000000*2))
	})

	It("is running a capture only on nodes specified by name (node-names)", func() {
		// Create unique namespace for this test
		namespace = createNamespace("test-nodenames")

		// create list of worker nodes
		workerNodes := make([]corev1.Node, 0)
		expectedCapturesOnNode := make(map[string]int)
		for _, node := range nodes {
			expectedCapturesOnNode[node.Name] = 0
			if _, exist := node.Labels["node-role.kubernetes.io/control-plane"]; !exist {
				workerNodes = append(workerNodes, node)
			}
		}

		// sample of worker nodes
		nodeNames := ""
		for i := 0; i < len(workerNodes)/2; i++ {
			expectedCapturesOnNode[workerNodes[i].Name] = 1
			nodeNames += "," + workerNodes[i].Name
		}
		nodeNames = strings.Trim(nodeNames, ",")

		// Create capture on nodes with specific labels.
		out := assertExecCommandOutput("kubectl", "retina", "capture", "create", fmt.Sprintf("--node-names=%s", nodeNames), fmt.Sprintf("--namespace=%s", namespace), fmt.Sprintf("--duration=%s", defaultDurationStr), "--host-path=/mnt/test", "--debug")
		captureName = getCaptureName(out)

		// Wait for all capture jobs to start running a pod before checking for captures
		waitUntilCapturePodsStart(captureName, namespace)

		for _, node := range nodes {
			capturePodList, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{FieldSelector: "spec.nodeName=" + node.Name, LabelSelector: fmt.Sprintf("%s=%s", label.AppLabel, captureConstants.CaptureAppname)})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(capturePodList.Items)).Should(Equal(expectedCapturesOnNode[node.Name]))
		}
	})

	It("is running a capture for the specified packet size (packet-size)", func() {
		// Create unique namespace for this test
		namespace = createNamespace("test-packet-size")

		// Create capture on nodes with specific labels.
		nodeSelector := fmt.Sprintf("kubernetes.io/hostname=%s", ictx.Nodes.Items[0].Name)
		out := assertExecCommandOutput("bash", "-c", fmt.Sprintf("kubectl retina capture create --node-selectors=%s --namespace=%s --duration=%s --packet-size=96 --host-path=/mnt/test --debug", nodeSelector, namespace, defaultDurationStr))

		// Obtain capture name to delete after test.
		captureName = getCaptureName(out)

		// Wait for all capture jobs to start running a pod before checking for captures
		waitUntilCapturePodsStart(captureName, namespace)
		capturePods, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{LabelSelector: "capture-name=" + captureName})
		Expect(err).ToNot(HaveOccurred())

		captureContainerExists := false
		for _, capturePod := range capturePods.Items {
			for _, container := range capturePod.Spec.Containers {
				if container.Name == captureConstants.CaptureAppname {
					captureContainerExists = true
					for _, envVar := range container.Env {
						if envVar.Name == captureConstants.PacketSizeEnvKey {
							Expect(strings.TrimSpace(envVar.Value)).To(Equal("96"))
						}
					}
				}
			}
		}

		Expect(captureContainerExists).To(Equal(true))
	})

	It("should not contain metadata (include-metadata)", func() {
		hostPath := "/mnt/test"
		namespace = createNamespace("test-metadata")

		// Create capture with metadata flag off
		nodeName := ictx.Nodes.Items[0].Name
		nodeSelector := fmt.Sprintf("kubernetes.io/hostname=%s", nodeName)
		_ = assertExecCommandOutput("bash", "-c", fmt.Sprintf("kubectl retina capture create --include-metadata=false --node-selectors=%s --namespace=%s --duration=%s --host-path=%s --debug", nodeSelector, namespace, defaultDurationStr, hostPath))

		// Obtain capture name to delete after test.
		out := assertExecCommandOutput("bash", "-c", fmt.Sprintf("kubectl retina capture list --namespace %s | awk '{print $2}'", namespace))
		captureName = getCaptureName(out)

		capturePod := corev1.Pod{}
		// Make sure all pods are completed
		Eventually(func() bool {
			capturePods, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{LabelSelector: "capture-name=" + captureName})
			// In case there's no pods created
			if len(capturePods.Items) == 0 {
				return false
			}
			if err != nil {
				return false
			}
			for _, capturePod := range capturePods.Items {
				if capturePod.Status.Phase != corev1.PodSucceeded {
					return false
				}
			}
			capturePod = capturePods.Items[0]
			return true
		}, 10*time.Minute, podRunningCheckInterval).Should(BeTrue())

		// copy capture file to local
		err, targetFilePath := copyCaptureFileToLocal(capturePod)
		Expect(err).ToNot(HaveOccurred())

		out = assertExecCommandOutput("bash", "-c", fmt.Sprintf("tar -ztvf %s ", targetFilePath))
		// checks if ip-resources.txt (a metadata file) was extracted to the folder
		Expect(strings.Contains(out, "ip-resources.txt")).To(Equal(false))
	})

	It("should return with prompt and does not create capture jobs when the job number limit reaches", func() {
		// Create unique namespace for this test
		namespace = createNamespace("test-nodenames")

		// create list of worker nodes
		workerNodes := make([]corev1.Node, 0)
		for _, node := range nodes {
			expectedCapturesOnNode[node.Name] = 0
			if _, exist := node.Labels["node-role.kubernetes.io/control-plane"]; !exist {
				workerNodes = append(workerNodes, node)
			}
		}

		jobNumLimit := 1

		// sample of worker nodes
		if len(workerNodes) < jobNumLimit+1 {
			Skip("Too few worker nodes to run job limit test")
		}
		nodeNames := ""
		for i := 0; i < jobNumLimit+1; i++ {
			nodeNames += "," + workerNodes[i].Name
		}
		nodeNames = strings.Trim(nodeNames, ",")

		command := exec.Command("kubectl", "retina", "capture", "create", fmt.Sprintf("--job-num-limit=%d", jobNumLimit), fmt.Sprintf("--node-names=%s", nodeNames), fmt.Sprintf("--namespace=%s", namespace), fmt.Sprintf("--duration=%s", defaultDurationStr), "--host-path=/mnt/test", "--debug")
		out, err := command.CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		captureJobNumExceedLimitError := CaptureJobNumExceedLimitError{
			CurrentNum: jobNumLimit + 1,
			Limit:      jobNumLimit,
		}
		Expect(string(out)).To(Equal(captureJobNumExceedLimitError.Error()))
	})
})

func bashCommand(command string) string {
	return assertExecCommandOutput("bash", "-c", command)
}

// waitUntilCapturePodsStart waits until capture pods start running
func waitUntilCapturePodsStart(captureName, namespace string) {
	// Make sure all pods are running
	Eventually(func() bool {
		capturePods, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{LabelSelector: "capture-name=" + captureName})
		// In case there's no pods created
		if len(capturePods.Items) == 0 {
			return false
		}
		if err != nil {
			return false
		}
		for _, capturePod := range capturePods.Items {
			if capturePod.Status.Phase != corev1.PodRunning {
				return false
			}
		}
		return true
	}, podRunningCheckDuration, podRunningCheckInterval).Should(BeTrue())
}

func getCaptureName(output string) string {
	re := regexp.MustCompile("retina-capture-[a-zA-Z0-9]+")
	captureName := re.FindStringSubmatch(string(output))[0]
	Expect(captureName).ToNot(BeEmpty())
	return captureName
}

func deleteCapture(captureName string, namespace string) {
	_ = assertExecCommandOutput("kubectl", "retina", "capture", "delete", "--name", captureName, "--namespace", namespace)
}

func createNamespace(namespace string) string {
	// Create unique namespace for each test
	namespaceUID := fmt.Sprintf("%s-%s", namespace, utilrand.String(5))
	_ = assertExecCommandOutput("kubectl", "create", "namespace", namespaceUID)
	return namespaceUID
}

func deleteNamespace(namespace string) {
	_ = assertExecCommandOutput("kubectl", "delete", "namespace", namespace)
}

func containsMap(subset map[string]string, superset map[string]string) bool {
	for k, v := range subset {
		if supersetValue, ok := superset[k]; !ok || supersetValue != v {
			return false
		}
	}
	return true
}

func fetchAndPrintDebugInfo(namespace string) {
	l.Info("Printing debug info for Capture", zap.String("namespace", namespace))
	pods, err := ictx.Clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		l.Info("Failed to list pods", zap.String("namespace", namespace), zap.Error(err))
		return
	}
	for _, pod := range pods.Items {
		l.Info("Printing logs for pod", zap.String("namespace", pod.Namespace), zap.String("pod", pod.Name))
		logs, err := ictx.Clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{}).Do(context.TODO()).Raw()
		if err != nil {
			l.Error("Failed to get logs for pod", zap.String("namespace", pod.Namespace), zap.String("pod", pod.Name), zap.Error(err))
		} else {
			l.Info("Printing log", zap.String("log", string(logs)))
		}

		l.Info("Printing events for pod", zap.String("namespace", pod.Namespace), zap.String("pod", pod.Name))
		podEvents, err := ictx.Clientset.CoreV1().Events(namespace).List(context.TODO(), v1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name),
		})
		if err != nil {
			l.Error("Failed to get events for pod", zap.String("namespace", pod.Namespace), zap.String("pod", pod.Name), zap.Error(err))
		} else {
			for _, podEvent := range podEvents.Items {
				l.Info("Pod event", zap.String("Type", podEvent.Type), zap.String("Reason", podEvent.Reason), zap.String("Message", podEvent.Message))
			}
		}
	}
}

// execCommandOutput prints the error message if the error is not nil and asserts the error is nil.
func assertExecCommandOutput(name string, arg ...string) string {
	command := exec.Command(name, arg...)
	out, err := command.CombinedOutput()
	if err != nil {
		l.Error("command failed", zap.String("command", command.String()), zap.String("output", string(out)), zap.Error(err))
	}
	Expect(err).ToNot(HaveOccurred())
	return string(out)
}

func copyCaptureFileToLocal(capturePod corev1.Pod) (error, string) {
	// get capture file from the Capture pod log
	logs, err := ictx.Clientset.CoreV1().Pods(capturePod.Namespace).GetLogs(capturePod.Name, &corev1.PodLogOptions{}).Do(context.TODO()).Raw()
	if err != nil {
		l.Error("Failed to get logs for pod", zap.String("namespace", capturePod.Namespace), zap.String("pod", capturePod.Name), zap.Error(err))
		return err, ""
	}
	captureName := capturePod.Labels[label.CaptureNameLabel]
	// regular expression to get the name of the zip file, starting with retina-capture- and end wth .tar.gz
	re := regexp.MustCompile(captureName + `.*\.tar\.gz`)
	zipFileName := strings.TrimLeft(re.FindString(string(logs)), "\n")
	if zipFileName == "" {
		return fmt.Errorf("failed to get zip file name from pod log"), ""
	}

	hostPath := capturePod.Spec.Containers[0].VolumeMounts[0].MountPath
	// copy folder from k8s node to local target folder
	podName := fmt.Sprintf("copy-file-from-node-capture-%s", captureName)
	if _, err := ictx.Clientset.CoreV1().Pods(capturePod.Namespace).Create(context.TODO(), &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "alpine",
					Image:   "mcr.microsoft.com/mirror/docker/library/alpine:3.16",
					Command: []string{"sleep", "infinity"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "host-path",
						MountPath: hostPath,
					}},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			Volumes: []corev1.Volume{{
				Name: "host-path",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: hostPath,
					},
				},
			}},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      corev1.LabelHostname,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{capturePod.Spec.NodeName},
									},
								},
							},
						},
					},
				},
			},
		},
	}, v1.CreateOptions{}); err != nil {
		return err, ""
	}

	if err := ictx.WaitForPodReady(namespace, podName); err != nil {
		return err, ""
	}

	targetFilePath := fmt.Sprintf("/tmp/%s.tar.gz", captureName)
	assertExecCommandOutput("bash", "-c", fmt.Sprintf("kubectl cp %s/%s:%s/%s %s", namespace, podName, hostPath, zipFileName, targetFilePath))
	return nil, targetFilePath
}
