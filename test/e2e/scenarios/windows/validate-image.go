package windows

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/microsoft/retina/test/e2e/common"
	k8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const retinaLabelSelector = "k8s-app=retina"

var (
	errNoWindowsNodes         = errors.New("no Windows nodes found")
	errWindowsBuildMismatch   = errors.New("windows node build mismatch")
	errWindowsContainerReady  = errors.New("retina container is not ready under containerd")
	errWindowsImageUnverified = errors.New("windows image verification failed")
)

type ValidateImage struct {
	KubeConfigFilePath string
	Years              []string
}

func (v *ValidateImage) Prevalidate() error {
	if len(v.Years) == 0 {
		return fmt.Errorf("%w: no Windows versions selected", common.ErrUnsupportedWindowsYear)
	}
	for _, year := range v.Years {
		if _, err := windowsBuild(year); err != nil {
			return err
		}
	}
	return nil
}

func (v *ValidateImage) Stop() error {
	return nil
}

func windowsBuild(year string) (string, error) {
	switch year {
	case "2022":
		return "10.0.20348", nil
	case "2025":
		return "10.0.26100", nil
	default:
		return "", fmt.Errorf("%w %q: use 2022 or 2025", common.ErrUnsupportedWindowsYear, year)
	}
}

func (v *ValidateImage) Run() error {
	builds := make([]string, 0, len(v.Years))
	for _, year := range v.Years {
		build, err := windowsBuild(year)
		if err != nil {
			return err
		}
		builds = append(builds, build)
	}
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("building kubeconfig: %w", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("creating Kubernetes client: %w", err)
	}
	ctx := context.TODO()
	return validateImages(ctx, client, builds, func(pod v1.Pod, command string) ([]byte, error) {
		return k8s.ExecPod(ctx, client, config, pod.Namespace, pod.Name, command)
	})
}

func validateImages(ctx context.Context, client kubernetes.Interface, builds []string, exec func(v1.Pod, string) ([]byte, error)) error {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "kubernetes.io/os=windows"})
	if err != nil {
		return fmt.Errorf("listing Windows nodes: %w", err)
	}
	if len(nodes.Items) == 0 {
		return fmt.Errorf("%w for builds %v validation", errNoWindowsNodes, builds)
	}
	pods, err := client.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{LabelSelector: retinaLabelSelector})
	if err != nil {
		return fmt.Errorf("listing Retina pods: %w", err)
	}
	seen := make(map[string]bool, len(builds))
	for i := range nodes.Items {
		node := &nodes.Items[i]
		build := node.Labels["node.kubernetes.io/windows-build"]
		if !slices.Contains(builds, build) {
			return fmt.Errorf("%w: node %s has Windows build %q, expected %s", errWindowsBuildMismatch, node.Name, build, strings.Join(builds, " or "))
		}
		seen[build] = true
		verified := false
		for j := range pods.Items {
			pod := &pods.Items[j]
			if pod.Spec.NodeName != node.Name {
				continue
			}
			for k := range pod.Status.ContainerStatuses {
				status := &pod.Status.ContainerStatuses[k]
				if !status.Ready || !strings.HasPrefix(status.ContainerID, "containerd://") {
					return fmt.Errorf("%w: %s/%s", errWindowsContainerReady, pod.Name, status.Name)
				}
				id := strings.TrimPrefix(status.ContainerID, "containerd://")
				output, err := exec(*pod, imageVersionCommand(id))
				if err != nil {
					return fmt.Errorf("checking Windows image on %s: %w: %s", node.Name, err, output)
				}
				version := strings.TrimSpace(string(output))
				if !strings.HasPrefix(version, build+".") {
					return fmt.Errorf("%w: node %s runs an image with os.version %q, expected %s.*", errWindowsImageUnverified, node.Name, version, build)
				}
				log.Printf("Windows build %s image verified on %s: os.version %s", build, node.Name, version)
				verified = true
			}
		}
		if !verified {
			return fmt.Errorf("%w: no running Retina container verified on Windows node %s", errWindowsImageUnverified, node.Name)
		}
	}
	for _, build := range builds {
		if !seen[build] {
			return fmt.Errorf("%w for required build %s", errNoWindowsNodes, build)
		}
	}
	return nil
}

func validateWorkloadNode(ctx context.Context, client kubernetes.Interface, namespace, podName, year string) (string, error) {
	build, err := windowsBuild(year)
	if err != nil {
		return "", err
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting WS%s workload %s/%s: %w", year, namespace, podName, err)
	}
	if pod.Spec.NodeName == "" {
		return "", fmt.Errorf("%w: %s/%s", k8s.ErrPodNotAssigned, namespace, podName)
	}
	node, err := client.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting WS%s workload node %s: %w", year, pod.Spec.NodeName, err)
	}
	if actual := node.Labels["node.kubernetes.io/windows-build"]; actual != build {
		return "", fmt.Errorf("%w: WS%s workload %s is on node %s with build %q, expected %s", errWindowsBuildMismatch, year, podName, node.Name, actual, build)
	}
	log.Printf("WS%s workload %s/%s verified on node %s (build %s)", year, namespace, podName, node.Name, build)
	return node.Name, nil
}

// imageVersionCommand prints the os.version of the image the container actually runs,
// which differs per Windows build even though both nodes pull the same image index.
func imageVersionCommand(containerID string) string {
	script := fmt.Sprintf(`$cri='C:\Program Files\containerd\crictl.exe';
$ep='npipe:////./pipe/containerd-containerd';
$c=& $cri --runtime-endpoint $ep inspect '%s' | ConvertFrom-Json;
(& $cri --runtime-endpoint $ep inspecti $c.status.imageRef | ConvertFrom-Json).info.imageSpec.'os.version'`, containerID)
	return powerShellCommand(script)
}

func powerShellCommand(script string) string {
	encoded := utf16.Encode([]rune(script))
	data := make([]byte, len(encoded)*2)
	for i, value := range encoded {
		binary.LittleEndian.PutUint16(data[i*2:], value)
	}
	return "powershell.exe -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(data)
}
