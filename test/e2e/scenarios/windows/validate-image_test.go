package windows

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	k8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
)

var errRuntimeImageVersion = errors.New("running image has unexpected os.version")

func TestImageCheckOnWindows(t *testing.T) {
	podName := os.Getenv("RETINA_WINDOWS_IMAGE_POD")
	if podName == "" {
		t.Skip("set RETINA_WINDOWS_IMAGE_POD, RETINA_WINDOWS_IMAGE_KUBECONFIG and RETINA_WINDOWS_IMAGE_YEAR to check an existing agent")
	}
	kubeconfig := os.Getenv("RETINA_WINDOWS_IMAGE_KUBECONFIG")
	require.NotEmpty(t, kubeconfig)
	year := os.Getenv("RETINA_WINDOWS_IMAGE_YEAR")
	build, err := windowsBuild(year)
	require.NoError(t, err)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	pod, err := client.CoreV1().Pods("kube-system").Get(ctx, podName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, pod.Status.ContainerStatuses, 1)
	status := pod.Status.ContainerStatuses[0]
	require.True(t, status.Ready)
	require.True(t, strings.HasPrefix(status.ContainerID, "containerd://"))
	id := strings.TrimPrefix(status.ContainerID, "containerd://")
	wrongBuild := "10.0.20348."
	if year == "2022" {
		wrongBuild = "10.0.26100."
	}
	for _, tc := range []struct {
		name, prefix string
		wantErr      bool
	}{
		{name: "matching running image and snapshot", prefix: build + "."},
		{name: "wrong image version rejected", prefix: wrongBuild, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := k8s.ExecPod(ctx, client, config, pod.Namespace, pod.Name, imageCheckCommand(id, tc.prefix))
			if tc.wantErr {
				require.Error(t, err, "%s", output)
				require.Contains(t, string(output), "unexpected os.version")
			} else {
				require.NoError(t, err, "%s", output)
				require.Contains(t, string(output), "RETINA_WINDOWS_IMAGE_VERIFIED")
			}
		})
	}
}

func TestValidateImages(t *testing.T) {
	const (
		build    = "10.0.26100"
		nodeName = "windows-test-node"
		podName  = "retina"
	)
	for _, tc := range []struct {
		name      string
		nodeBuild string
		noNode    bool
		noPod     bool
		ready     bool
		output    string
		execErr   error
		wantErr   string
	}{
		{name: "matching WS2025 running image", nodeBuild: build, ready: true, output: "RETINA_WINDOWS_IMAGE_VERIFIED 10.0.26100.33674 sha256:config"},
		{name: "wrong host version", nodeBuild: "10.0.20348", ready: true, wantErr: "expected 10.0.26100"},
		{name: "missing Windows node", noNode: true, wantErr: "no Windows nodes"},
		{name: "missing Retina pod", nodeBuild: build, noPod: true, wantErr: "no running Retina container"},
		{name: "unready Retina", nodeBuild: build, wantErr: "not ready under containerd"},
		{name: "runtime rejects selected image", nodeBuild: build, ready: true, execErr: errRuntimeImageVersion, wantErr: "unexpected os.version"},
		{name: "empty runtime response", nodeBuild: build, ready: true, wantErr: "no confirmation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientset()
			ctx := context.Background()
			if !tc.noNode {
				_, err := client.CoreV1().Nodes().Create(ctx, &v1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: nodeName, Labels: map[string]string{
						"kubernetes.io/os": "windows", "node.kubernetes.io/windows-build": tc.nodeBuild,
					}},
				}, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			if !tc.noPod {
				_, err := client.CoreV1().Pods("kube-system").Create(ctx, &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: podName, Labels: map[string]string{"k8s-app": podName}},
					Spec:       v1.PodSpec{NodeName: nodeName},
					Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
						{Name: podName, Ready: tc.ready, ContainerID: "containerd://test-container"},
					}},
				}, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			err := validateImages(ctx, client, build, func(_ v1.Pod, command string) ([]byte, error) {
				require.Contains(t, command, "powershell.exe -NoProfile -NonInteractive -EncodedCommand ")
				return []byte(tc.output), tc.execErr
			})
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
