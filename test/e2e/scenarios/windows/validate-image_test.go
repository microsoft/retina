package windows

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	windows2022Build = "10.0.20348"
	windows2025Build = "10.0.26100"
)

func addWindowsNodeWithAgent(t *testing.T, client kubernetes.Interface, name, build string, ready bool) {
	t.Helper()
	_, err := client.CoreV1().Nodes().Create(t.Context(), &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
			"kubernetes.io/os": "windows", "node.kubernetes.io/windows-build": build,
		}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = client.CoreV1().Pods("kube-system").Create(t.Context(), &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "retina-" + name, Labels: map[string]string{"k8s-app": "retina"}},
		Spec:       v1.PodSpec{NodeName: name},
		Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
			{Name: "retina", Ready: ready, ContainerID: "containerd://" + name},
		}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestValidateImages(t *testing.T) {
	for _, tc := range []struct {
		name, nodeBuild, osVersion, wantErr string
		ready                               bool
	}{
		{name: "matching WS2025 image", nodeBuild: windows2025Build, osVersion: "10.0.26100.33674", ready: true},
		{name: "WS2022 image on WS2025 node", nodeBuild: windows2025Build, osVersion: "10.0.20348.5873", ready: true, wantErr: "os.version"},
		{name: "wrong host version", nodeBuild: windows2022Build, ready: true, wantErr: "expected 10.0.26100"},
		{name: "unready Retina", nodeBuild: windows2025Build, wantErr: "not ready under containerd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientset()
			addWindowsNodeWithAgent(t, client, "node", tc.nodeBuild, tc.ready)
			err := validateImages(t.Context(), client, []string{windows2025Build}, func(v1.Pod, string) ([]byte, error) {
				return []byte(tc.osVersion + "\r\n"), nil
			})
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateImagesRequiresEverySelectedBuild(t *testing.T) {
	client := fake.NewClientset()
	addWindowsNodeWithAgent(t, client, "node22", windows2022Build, true)
	err := validateImages(t.Context(), client, []string{windows2022Build, windows2025Build}, func(v1.Pod, string) ([]byte, error) {
		return []byte("10.0.20348.5873"), nil
	})
	require.ErrorContains(t, err, "no Windows nodes found for required build 10.0.26100")
}

func TestValidateWorkloadNode(t *testing.T) {
	for _, tc := range []struct {
		name, year, build, wantErr string
	}{
		{name: "WS2022", year: "2022", build: windows2022Build},
		{name: "WS2025", year: "2025", build: windows2025Build},
		{name: "wrong pool", year: "2025", build: windows2022Build, wantErr: "windows node build mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientset(
				&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", Labels: map[string]string{v1.LabelWindowsBuild: tc.build}}},
				&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "workload", Namespace: "default"}, Spec: v1.PodSpec{NodeName: "node"}},
			)
			nodeName, err := validateWorkloadNode(t.Context(), client, "default", "workload", tc.year)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, "node", nodeName)
			}
		})
	}
}

func TestDeleteWorkloadPod(t *testing.T) {
	client := fake.NewClientset(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "non-hpc-pod-2022", Namespace: "default"}})

	require.NoError(t, deleteWorkloadPod(t.Context(), client, "default", "non-hpc-pod-2022"))
	_, err := client.CoreV1().Pods("default").Get(t.Context(), "non-hpc-pod-2022", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	require.NoError(t, deleteWorkloadPod(t.Context(), client, "default", "non-hpc-pod-2022"))
}
