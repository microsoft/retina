package scaletest

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCountPlatformPods(t *testing.T) {
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "linux-node",
				Labels: map[string]string{
					corev1.LabelOSStable:   "linux",
					corev1.LabelArchStable: "amd64",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "windows-node",
				Labels: map[string]string{
					corev1.LabelOSStable:   "windows",
					corev1.LabelArchStable: "amd64",
				},
			},
		},
	}
	pods := []corev1.Pod{
		{Spec: corev1.PodSpec{NodeName: "linux-node"}},
		{
			Spec: corev1.PodSpec{NodeName: "windows-node"},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
				{RestartCount: 2},
			}},
		},
		{Spec: corev1.PodSpec{NodeName: "unknown-node"}},
	}

	linux, windows, unknown, restarts := countPlatformPods(pods, nodes)
	require.Equal(t, 1, linux)
	require.Equal(t, 1, windows)
	require.Equal(t, 1, unknown)
	require.Equal(t, 2, restarts)
}
