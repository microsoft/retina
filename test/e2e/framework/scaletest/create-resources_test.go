package scaletest

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestGeneratePlatformDeployments(t *testing.T) {
	generator := &CreateResources{
		RealPodType:                  "kapinger",
		NumRealReplicas:              20,
		NumUniqueLabelsPerDeployment: 1,
	}

	for _, os := range []string{"linux", "windows"} {
		t.Run(os, func(t *testing.T) {
			objects := generator.generateDeployments(os, 2)
			require.Len(t, objects, 2)

			deployment, ok := objects[0].(*appsv1.Deployment)
			require.True(t, ok)
			require.Equal(t, "kapinger-"+os+"-dep-00000", deployment.Name)
			require.Equal(t, os, deployment.Spec.Template.Spec.NodeSelector[corev1.LabelOSStable])
			require.Equal(t, "amd64", deployment.Spec.Template.Spec.NodeSelector[corev1.LabelArchStable])
			require.Equal(t, "true", deployment.Spec.Template.Spec.NodeSelector["scale-test"])
			require.Equal(t, os, deployment.Spec.Template.Labels[TrafficOSLabel])
			require.Equal(t, int32(20), *deployment.Spec.Replicas)
		})
	}
}

func TestGeneratePlatformServices(t *testing.T) {
	generator := &CreateResources{RealPodType: "kapinger"}

	for _, os := range []string{"linux", "windows"} {
		t.Run(os, func(t *testing.T) {
			objects := generator.generateServices(os, 2)
			require.Len(t, objects, 2)

			service, ok := objects[0].(*corev1.Service)
			require.True(t, ok)
			require.Equal(t, "kapinger-"+os+"-svc-00000", service.Name)
			require.Equal(t, os, service.Labels[TrafficOSLabel])
			require.Equal(t, "kapinger-"+os+"-dep-00000", service.Spec.Selector["name"])
			require.Equal(t, "kapinger", service.Labels["app"])
		})
	}
}
