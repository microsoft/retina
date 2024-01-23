// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

// KubeManager emulates the KubeManager in upstream NetPol e2e:
// https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/netpol/kubemanager.go

package integration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nwv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/scheme"
	netutils "k8s.io/utils/net"
)

const probeTimeoutSeconds = 3

var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
	ErrNoServiceIP         = errors.New("could not find service IP for given Pod")
)

// TestPod represents an actual running pod. For each Pod defined by the model,
// there will be a corresponding TestPod. TestPod includes some runtime info
// (namespace name, service IP) which is not available in the model.
type TestPod struct {
	Namespace  string
	Name       string
	Containers []string
	ServiceIP  string
}

type KubeManager struct {
	l              *log.ZapLogger
	clientSet      clientset.Interface
	crdClient      apiextensionsv1beta1.ApiextensionsV1beta1Client
	namespaceNames []string
	allPods        []TestPod
}

func NewKubeManager(l *log.ZapLogger, clientSet clientset.Interface) *KubeManager {
	return &KubeManager{
		l:         l.Named("kube-manager"),
		clientSet: clientSet,
	}
}

func (k *KubeManager) InitializeClusterFromModel(namespaces ...*ModelNamespace) error {
	var createdPods []*v1.Pod
	for _, ns := range namespaces {
		err := k.createNamespace(ns.BaseName)
		if err != nil {
			return err
		}

		k.namespaceNames = append(k.namespaceNames, ns.BaseName)

		for _, pod := range ns.Pods {
			k.l.Info(fmt.Sprintf("creating pod %s/%s with matching service", ns.BaseName, pod.Name), zap.Any("pod", pod))

			kubePod, err := k.createPod(pod.KubePod(ns.BaseName))
			if err != nil {
				return err
			}

			createdPods = append(createdPods, kubePod)
			svc, err := k.createService(pod.Service(ns.BaseName))
			if err != nil {
				return err
			}
			if netutils.ParseIPSloppy(svc.Spec.ClusterIP) == nil {
				return fmt.Errorf("empty IP address found for service %s/%s", svc.Namespace, svc.Name)
			}

			k.allPods = append(k.allPods, TestPod{
				Namespace: kubePod.Namespace,
				Name:      kubePod.Name,
				ServiceIP: svc.Spec.ClusterIP,
			})
		}
	}

	for _, createdPod := range createdPods {
		err := k.waitForPodReady(createdPod)
		if err != nil {
			return fmt.Errorf("unable to wait for pod %s/%s. err: %w", createdPod.Namespace, createdPod.Name, err)
		}
	}

	for _, ns := range namespaces {
		k.logOutputOfKubectlCommand("get", "pod", "-n", ns.BaseName, "-o", "wide", "--show-labels")
		k.logOutputOfKubectlCommand("get", "svc", "-n", ns.BaseName, "-o", "wide", "--show-labels")
	}

	return nil
}

// CleanupNamespaces will cleanup all created objects.
// This should be called after done with testing.
func (k *KubeManager) CleanupNamespaces() {
	for _, ns := range k.namespaceNames {
		if err := k.clientSet.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{}); err != nil {
			k.l.Error("failed to delete namespace", zap.String("ns", ns), zap.String("err", err.Error()))
		}
	}
}

// Probe execs into a pod and probes another pod once.
func (k *KubeManager) Probe(fromNs, fromPod, toNs, toPod string, proto ModelProtocol, toPort int) error {
	return k.ProbeRepeatedly(fromNs, fromPod, toNs, toPod, proto, toPort, 0)
}

// ProbeRepeatedlyKind will make several kubectl exec calls (one per probe) until secondsToProbe has passed
func (k *KubeManager) ProbeRepeatedlyKind(fromNs, fromPod, toNs, toPod string, proto ModelProtocol, toPort, secondsToProbe int) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	finish := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(1 * time.Millisecond)
		defer ticker.Stop()
		round := 1
		for {
			select {
			case <-ticker.C:
				k.l.Info(fmt.Sprintf("probing round %5d", round))
				round++
				k.ProbeRepeatedly(fromNs, fromPod, toNs, toPod, proto, toPort, 0)
			case <-finish:
				k.l.Info("finished probing")
				wg.Done()
				return
			}
		}
	}()

	time.Sleep(time.Duration(secondsToProbe) * time.Second)
	finish <- struct{}{}
	wg.Wait()
}

// ProbeRepeatedly execs into a pod once.
// If secondsToProbe <= 0 seconds, then this method will tell the Pod to probe once.
// Otherwise, it will tell the Pod to run a loop, probing until secondsToProbe has passed (loop does not work in Kind).
func (k *KubeManager) ProbeRepeatedly(fromNs, fromPod, toNs, toPod string, proto ModelProtocol, toPort, secondsToProbe int) error {
	var toIP string
	for _, pod := range k.allPods {
		if pod.Namespace == toNs && pod.Name == toPod {
			toIP = pod.ServiceIP
			break
		}
	}
	if toIP == "" {
		return ErrNoServiceIP
	}

	var cmd []string
	agnhostTimeout := fmt.Sprintf("--timeout=%ds", probeTimeoutSeconds)
	switch proto {
	case TCP:
		cmd = []string{"/agnhost", "connect", net.JoinHostPort(toIP, fmt.Sprint(toPort)), agnhostTimeout, "--protocol=tcp"}
	case UDP:
		cmd = []string{"/agnhost", "connect", net.JoinHostPort(toIP, fmt.Sprint(toPort)), agnhostTimeout, "--protocol=udp"}
	case HTTP:
		cmd = []string{"curl", net.JoinHostPort(toIP, fmt.Sprint(toPort)), "--connect-timeout", fmt.Sprint(probeTimeoutSeconds)}
	default:
		k.l.Error("probing protocol not supported", zap.Any("proto", proto))
		return ErrUnsupportedProtocol
	}

	// works on AKS but not in Kind
	if secondsToProbe >= 1 {
		// break out of infinite for-loop once enough seconds have passed
		loop := fmt.Sprintf("'start=`date +%%s` ; while : ; do %s ; curr=`date +%%s` ; if [[ $(( curr - start )) -ge %d ]]; then break ; fi ; done'",
			strings.Join(cmd, " "), secondsToProbe)
		cmd = []string{"bash", "-c", loop}
	}

	cmd = append([]string{"kubectl", "exec", fromPod, "-n", fromNs, "--"}, cmd...)

	k.l.Info("Starting probe", zap.String("fromNs", fromNs), zap.String("fromPod", fromPod), zap.String("toNs", toNs), zap.String("toPod", toPod), zap.String("toIP", toIP))
	k.l.Info(fmt.Sprintf("executing command: %s", strings.Join(cmd, " ")))

	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	k.l.Info(string(output))
	if err != nil {
		return fmt.Errorf("seems like we failed to run the probe command. err: %w", err)
	}
	return nil
}

// Perform nslookup from a pod to a domain.
func (k *KubeManager) PerformNslookup(fromNs, fromPod, domain string) error {
	// construct nslookup command
	cmd := []string{"nslookup", domain}
	cmd = append([]string{"kubectl", "exec", fromPod, "-n", fromNs, "--"}, cmd...)

	k.l.Info("Starting nslookup", zap.String("fromNs", fromNs), zap.String("fromPod", fromPod), zap.String("domain", domain))
	k.l.Info(fmt.Sprintf("executing command: %s", strings.Join(cmd, " ")))

	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	k.l.Info(string(output))
	if err != nil {
		return fmt.Errorf("seems like we failed to run the nslookup command. err: %w", err)
	}
	return nil
}

func (k *KubeManager) createNamespace(name string) error {
	labels := map[string]string{
		"e2e": "true",
	}

	namespaceObj := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: v1.NamespaceStatus{},
	}

	// retry a few times
	return wait.PollImmediateWithContext(context.TODO(), 1*time.Minute, 30*time.Second, func(ctx context.Context) (bool, error) {
		_, err := k.clientSet.CoreV1().Namespaces().Create(ctx, namespaceObj, metav1.CreateOptions{})
		if err != nil {
			k.l.Error("unable to create namespace", zap.String("err", err.Error()))
			return false, nil
		}
		return true, nil
	})
}

// createService is a convenience function for service setup.
func (k *KubeManager) createService(service *v1.Service) (*v1.Service, error) {
	ns := service.Namespace
	name := service.Name

	k.l.Info(fmt.Sprintf("creating svc %s/%s", ns, name))
	createdService, err := k.clientSet.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create service %s/%s. err: %w", ns, name, err)
	}
	return createdService, nil
}

// createPod is a convenience function for pod setup.
func (k *KubeManager) createPod(pod *v1.Pod) (*v1.Pod, error) {
	ns := pod.Namespace
	k.l.Info(fmt.Sprintf("creating pod %s/%s", ns, pod.Name))

	createdPod, err := k.clientSet.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create pod %s/%s. err: %w", ns, pod.Name, err)
	}
	return createdPod, nil
}

func (k *KubeManager) waitForPodReady(pod *v1.Pod) error {
	ctx, cancelFn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelFn()

	for {
		select {
		case <-ctx.Done():
			k.logOutputOfKubectlCommand("get", "pods", "-n", pod.Namespace, "-o", "wide", "--show-labels")

			k.logOutputOfKubectlCommand("get", "node", "-o", "wide", "--show-labels")

			k.logOutputOfKubectlCommand("describe", "pod", "-n", pod.Namespace, pod.Name)

			return fmt.Errorf("timed out waiting for pod %s to be ready", pod.Name)
		default:
			pod, err := k.clientSet.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
			if err != nil {
				// Maybe intermittent error, retry.
				continue
			}
			if pod.Status.Phase == corev1.PodRunning {
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (k *KubeManager) logOutputOfKubectlCommand(args ...string) {
	k.l.Info(fmt.Sprintf("running command: kubectl %s", strings.Join(args, " ")))
	out, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		k.l.Error("failed to run kubectl command", zap.String("err", err.Error()))
		return
	}

	k.l.Info(string(out))
}

func (km *KubeManager) RestartDaemonset(namespace string, name string) error {
	km.l.Info("Restarting daemonset", zap.String("namespace", namespace), zap.String("name", name))
	execCmd := []string{"kubectl", "rollout", "restart", "daemonset", name, "-n", namespace}
	cmd := exec.Command(execCmd[0], execCmd[1:]...)
	out, err := cmd.Output()
	if err != nil {
		km.l.Error("Failed to restart daemonset", zap.Error(err), zap.String("output", string(out)))
		return err
	}
	// sleep for a while to let the daemonset restart.
	time.Sleep(5 * time.Second)
	return nil
}

// func to check daemonset is ready
func (km *KubeManager) WaitForDaemonsetReady(namespace string, name string) error {
	km.l.Info("Waiting for daemonset to be ready", zap.String("namespace", namespace), zap.String("name", name))
	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for daemonset %s to be ready", name)
		default:
			ds, err := km.clientSet.AppsV1().DaemonSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				// Maybe intermittent error, retry.
				time.Sleep(1 * time.Second)
				continue
			}
			// waits for daemonset pods to be running
			if ds.Status.DesiredNumberScheduled == ds.Status.NumberReady && ds.Status.DesiredNumberScheduled == ds.Status.NumberAvailable {
				km.l.Info("Daemonset is ready", zap.String("namespace", namespace), zap.String("name", name))
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// Update the configmap to enable pod level metrics.
func (km *KubeManager) UpdateRetinaConfigMap(kv map[string]string, configmap string) error {
	km.l.Info("Updating configmap", zap.String("name", configmap), zap.Any("new key values", kv))
	cm, err := km.clientSet.CoreV1().ConfigMaps("kube-system").Get(context.Background(), configmap, metav1.GetOptions{})
	if err != nil {
		km.l.Error("Failed to get configmap", zap.Error(err))
		return err
	}
	cfgstring := strings.ReplaceAll(cm.Data["config.yaml"], "\\n", "\n")
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(cfgstring), &data); err != nil {
		km.l.Error("Failed to unmarshal configmap", zap.Error(err), zap.Any("kcf", data))
		return err
	}
	for k, v := range kv {
		data[k] = v
	}
	kcfgBytes, err := yaml.Marshal(data)
	if err != nil {
		km.l.Error("Failed to marshal configmap", zap.Error(err))
		return err
	}
	cm.Data["config.yaml"] = string(kcfgBytes)
	_, err = km.clientSet.CoreV1().ConfigMaps("kube-system").Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		km.l.Error("Failed to update configmap", zap.Error(err))
		return err
	}
	return nil
}

// SetPodLevel sets the pod level metrics to enabled/disabled.
// It updates the configmap cmName and restarts the daemonset dsName in namespace.
// dsName should be retina-agent or retina-agent-win
func (km *KubeManager) SetPodLevel(enabled bool, cmName, namespace, dsName string) error {
	km.l.Info("Setting enablePodLevel", zap.Bool("enabled", enabled), zap.String("configmap", cmName))
	kv := map[string]string{
		"enablePodLevel": strconv.FormatBool(enabled),
	}
	err := km.UpdateRetinaConfigMap(kv, cmName)
	if err != nil {
		km.l.Error("Failed to update configmap", zap.Error(err))
		return err
	}
	err = km.RestartDaemonset(namespace, dsName)
	if err != nil {
		return err
	}
	err = km.WaitForDaemonsetReady(namespace, dsName)
	if err != nil {
		return err
	}
	return nil
}

func (km *KubeManager) CreateCRDFromFile(path string) error {
	crdYaml, err := os.ReadFile(path)
	if err != nil {
		km.l.Error("Failed to read crd yaml", zap.Error(err))
		return err
	}

	// Decode the CRD manifest into an Unstructured object
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(crdYaml, nil, nil)
	if err != nil {
		return err
	}
	crd, ok := obj.(*v1beta1.CustomResourceDefinition)
	if !ok {
		km.l.Error("Failed to convert CRD fixture to CRD", zap.Error(err))
		return err
	}

	_, err = km.crdClient.CustomResourceDefinitions().Create(context.Background(), crd, metav1.CreateOptions{})
	if err != nil {
		if statusError, isStatus := err.(*apierrors.StatusError); isStatus {
			if statusError.ErrStatus.Code == http.StatusConflict {
				km.l.Info("CRD already exists")
				return nil
			}
		}
		km.l.Error("Failed to create crd", zap.Error(err))
		return err
	}

	return nil
}

// annotate namespace
// To annotate with retina observe annotations, use common.RetinaObserveAnnotations and common.RetinaObserveAnnotationValue
func (km *KubeManager) AnnotateNamespace(ns string, annotations map[string]string) error {
	// uses client-go to annotate a namespace with the name ns
	km.l.Info("Annotating namespace", zap.String("namespace", ns))
	namespace, err := km.clientSet.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
	if err != nil {
		km.l.Error("Failed to get namespace", zap.Error(err))
		return err
	}
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		namespace.Annotations[k] = v
	}
	_, err = km.clientSet.CoreV1().Namespaces().Update(context.Background(), namespace, metav1.UpdateOptions{})
	if err != nil {
		km.l.Error("Failed to update namespace", zap.Error(err))
		return err
	}
	return nil
}

// annotate pod in a specific namespace with retina observe
func (km *KubeManager) AnnotatePod(pod, namespace string, annotations map[string]string) error {
	km.l.Info("Annotating pod", zap.String("pod", pod))
	podObj, err := km.clientSet.CoreV1().Pods(namespace).Get(context.Background(), pod, metav1.GetOptions{})
	if err != nil {
		km.l.Error("Failed to get pod", zap.Error(err))
		return err
	}
	if podObj.Annotations == nil {
		podObj.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		podObj.Annotations[k] = v
	}
	_, err = km.clientSet.CoreV1().Pods(namespace).Update(context.Background(), podObj, metav1.UpdateOptions{})
	if err != nil {
		km.l.Error("Failed to update pod", zap.Error(err))
		return err
	}
	return nil
}

// get namespaces that are annotated with annotations
func (km *KubeManager) GetAnnotatedNamespaces(annotations map[string]string) []*corev1.Namespace {
	km.l.Info("Getting annotated namespaces", zap.Any("annotations", annotations))
	// if no annotations are provided, return nil
	if len(annotations) == 0 {
		km.l.Warn("No annotations provided")
		return nil
	}
	namespaces, err := km.clientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		km.l.Error("Failed to get namespaces", zap.Error(err))
		return nil
	}

	var annotatedNamespaces []*corev1.Namespace
	for _, ns := range namespaces.Items {
		matches := true
		for k, v := range annotations {
			if nsValue, exists := ns.Annotations[k]; !exists || nsValue != v {
				matches = false
				break
			}
		}
		if matches {
			annotatedNamespaces = append(annotatedNamespaces, &ns)
		}
	}

	return annotatedNamespaces
}

// get annotated pods in a specific namespace with retina observe
func (km *KubeManager) GetAnnotatedPods(namespace string, annotations map[string]string) []*corev1.Pod {
	km.l.Info("Getting annotated pods", zap.String("namespace", namespace), zap.Any("annotations", annotations))
	pods, err := km.clientSet.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		km.l.Error("Failed to get pods", zap.Error(err))
		return nil
	}

	var annotatedPods []*corev1.Pod
	for _, pod := range pods.Items {
		matches := true
		for k, v := range annotations {
			if val, ok := pod.Annotations[k]; !ok || val != v {
				matches = false
				break
			}
		}
		if matches {
			annotatedPods = append(annotatedPods, &pod)
		}
	}
	return annotatedPods
}

func (km *KubeManager) RemovePodAnnotations(pod, namespace string, annotations map[string]string) error {
	km.l.Info("Removing pod annotations", zap.String("pod", pod), zap.String("namespace", namespace), zap.Any("annotations", annotations))
	podObj, err := km.clientSet.CoreV1().Pods(namespace).Get(context.Background(), pod, metav1.GetOptions{})
	if err != nil {
		km.l.Error("Failed to get pod", zap.Error(err))
		return err
	}
	for k := range annotations {
		delete(podObj.Annotations, k)
	}
	_, err = km.clientSet.CoreV1().Pods(namespace).Update(context.Background(), podObj, metav1.UpdateOptions{})
	if err != nil {
		km.l.Error("Failed to update pod", zap.Error(err))
		return err
	}
	return nil
}

func (km *KubeManager) RemoveNamespaceAnnotations(namespace string, annotations map[string]string) error {
	km.l.Info("Removing namespace annotations", zap.String("namespace", namespace), zap.Any("annotations", annotations))
	namespaceObj, err := km.clientSet.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		km.l.Error("Failed to get namespace", zap.Error(err))
		return err
	}
	for k := range annotations {
		delete(namespaceObj.Annotations, k)
	}
	_, err = km.clientSet.CoreV1().Namespaces().Update(context.Background(), namespaceObj, metav1.UpdateOptions{})
	if err != nil {
		km.l.Error("Failed to update namespace", zap.Error(err))
		return err
	}
	return nil
}

// ApplyNetworkDropPolicy applies drop ingress network policy on the given name in the given namespace.
func (km *KubeManager) ApplyNetworkDropPolicy(namespace, policyName, podLabelKey, podLabelValue string) error {
	_, err := km.clientSet.NetworkingV1().NetworkPolicies(namespace).Create(context.Background(), &nwv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
		Spec: nwv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					podLabelKey: podLabelValue,
				},
			},
			Ingress: []nwv1.NetworkPolicyIngressRule{},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		// Check if error is because the policy already exists
		if k8serrors.IsAlreadyExists(err) {
			km.l.Warn("Network policy already exists", zap.String("policyName", policyName))
			return nil
		}

		km.l.Error("Failed to create network policy", zap.Error(err))
		return err
	}

	km.l.Info("Network policy is created", zap.String("policyName", policyName))
	return nil
}

// RemoveNetworkPolicy removes the network policy with the given name in the given namespace.
func (km *KubeManager) RemoveNetworkPolicy(namespace, policyName string) error {
	err := km.clientSet.NetworkingV1().NetworkPolicies(namespace).Delete(context.Background(), policyName, metav1.DeleteOptions{})
	if err != nil {
		km.l.Error("Failed to delete network policy", zap.Error(err))
		return err
	}

	km.l.Info("Network policy is deleted", zap.String("policyName", policyName))
	return nil
}
