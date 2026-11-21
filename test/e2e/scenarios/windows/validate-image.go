package windows

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode/utf16"

	k8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const retinaLabelSelector = "k8s-app=retina"

var (
	errUnsupportedWindowsYear = errors.New("unsupported Windows year")
	errNoWindowsNodes         = errors.New("no Windows nodes found")
	errWindowsBuildMismatch   = errors.New("windows node build mismatch")
	errWindowsContainerReady  = errors.New("retina container is not ready under containerd")
	errWindowsImageUnverified = errors.New("windows image verification failed")
)

type ValidateImage struct {
	KubeConfigFilePath string
	Year               string
}

func (v *ValidateImage) Prevalidate() error {
	_, err := windowsBuild(v.Year)
	return err
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
		return "", fmt.Errorf("%w %q: use 2022 or 2025", errUnsupportedWindowsYear, year)
	}
}

func (v *ValidateImage) Run() error {
	build, err := windowsBuild(v.Year)
	if err != nil {
		return err
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
	return validateImages(ctx, client, build, func(pod v1.Pod, command string) ([]byte, error) {
		return k8s.ExecPod(ctx, client, config, pod.Namespace, pod.Name, command)
	})
}

func validateImages(ctx context.Context, client kubernetes.Interface, build string, exec func(v1.Pod, string) ([]byte, error)) error {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "kubernetes.io/os=windows"})
	if err != nil {
		return fmt.Errorf("listing Windows nodes: %w", err)
	}
	if len(nodes.Items) == 0 {
		return fmt.Errorf("%w for build %s validation", errNoWindowsNodes, build)
	}
	pods, err := client.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{LabelSelector: retinaLabelSelector})
	if err != nil {
		return fmt.Errorf("listing Retina pods: %w", err)
	}
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if actual := node.Labels["node.kubernetes.io/windows-build"]; actual != build {
			return fmt.Errorf("%w: node %s has Windows build %q, expected %s", errWindowsBuildMismatch, node.Name, actual, build)
		}
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
				// EncodedCommand keeps PowerShell quoting intact through ExecPod's argument splitting.
				command := imageCheckCommand(id, build+".")
				output, err := exec(*pod, command)
				if err != nil {
					return fmt.Errorf("checking Windows image on %s: %w: %s", node.Name, err, output)
				}
				if !strings.Contains(string(output), "RETINA_WINDOWS_IMAGE_VERIFIED") {
					return fmt.Errorf("%w: image verification on %s returned no confirmation: %s", errWindowsImageUnverified, node.Name, output)
				}
				log.Printf("Windows build %s image verified on %s: %s", build, node.Name, output)
				verified = true
			}
		}
		if !verified {
			return fmt.Errorf("%w: no running Retina container verified on Windows node %s", errWindowsImageUnverified, node.Name)
		}
	}
	return nil
}

func imageCheckCommand(containerID, buildPrefix string) string {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := fmt.Sprintf(`$ErrorActionPreference='Stop';
$cri='C:\Program Files\containerd\crictl.exe';
$ctr='C:\Program Files\containerd\ctr.exe';
$container=& $cri --runtime-endpoint npipe:////./pipe/containerd-containerd inspect %s;
if ($LASTEXITCODE -ne 0) { throw 'crictl inspect failed' };
$container=$container | ConvertFrom-Json;
$image=& $cri --runtime-endpoint npipe:////./pipe/containerd-containerd inspecti $container.status.imageRef;
if ($LASTEXITCODE -ne 0) { throw 'crictl inspecti failed' };
$image=$image | ConvertFrom-Json;
if (!$image.info.imageSpec.'os.version' -or !$image.info.imageSpec.'os.version'.StartsWith(%s)) { throw 'Running image has unexpected os.version' };
$snapshot=& $ctr -n k8s.io containers info %s;
if ($LASTEXITCODE -ne 0) { throw 'ctr containers info failed' };
$snapshot=$snapshot | ConvertFrom-Json;
$layer=& $ctr -n k8s.io snapshots --snapshotter windows info $snapshot.SnapshotKey;
if ($LASTEXITCODE -ne 0) { throw 'ctr snapshots info failed' };
$layer=$layer | ConvertFrom-Json;
if (!$image.info.chainID -or $layer.Parent -ne $image.info.chainID) { throw 'Running snapshot does not match inspected image' };
Write-Output ('RETINA_WINDOWS_IMAGE_VERIFIED '+$image.info.imageSpec.'os.version'+' '+$image.status.id);`,
		quote(containerID), quote(buildPrefix), quote(containerID))
	encoded := utf16.Encode([]rune(script))
	data := make([]byte, len(encoded)*2)
	for i, value := range encoded {
		binary.LittleEndian.PutUint16(data[i*2:], value)
	}
	return "powershell.exe -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(data)
}
