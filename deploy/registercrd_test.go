package deploy

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	apiextv1fake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

// This test exists to ensure that the YAML files are present in the manifests directory,
// as the dockerfile build and copy depends on them, and matching the names
func TestGetCRD(t *testing.T) {
	base := "/manifests/controller/helm/retina/crds/%s"
	pwd, _ := os.Getwd()
	full := pwd + base
	require.FileExists(t, fmt.Sprintf(full, RetinaCapturesYAMLpath))
	require.FileExists(t, fmt.Sprintf(full, RetinaEndpointsYAMLpath))
	require.FileExists(t, fmt.Sprintf(full, MetricsConfigurationYAMLpath))
	require.FileExists(t, fmt.Sprintf(full, TracesConfigurationYAMLpath))

	capture, err := GetRetinaCapturesCRD()
	require.NoError(t, err)
	require.NotNil(t, capture)
	require.NotEmpty(t, capture.TypeMeta.Kind)

	endpoint, err := GetRetinaEndpointCRD()
	require.NoError(t, err)
	require.NotNil(t, endpoint)
	require.NotEmpty(t, endpoint.TypeMeta.Kind)

	metrics, err := GetRetinaMetricsConfigurationCRD()
	require.NoError(t, err)
	require.NotNil(t, metrics)
	require.NotEmpty(t, metrics.TypeMeta.Kind)

	traces, err := GetRetinaTracesConfigurationCRD()
	require.NoError(t, err)
	require.NotNil(t, traces)
	require.NotEmpty(t, traces.TypeMeta.Kind)
}

func TestInstallOrUpdateCRDs(t *testing.T) {
	capture, _ := GetRetinaCapturesCRD()
	endpoint, _ := GetRetinaEndpointCRD()
	metrics, _ := GetRetinaMetricsConfigurationCRD()
	traces, _ := GetRetinaTracesConfigurationCRD()

	tests := []struct {
		name                 string
		enableRetinaEndpoint bool
		want                 map[string]*apiextensionsv1.CustomResourceDefinition
		wantErr              bool
	}{
		{
			name:                 "install all CRDs",
			enableRetinaEndpoint: true,
			want: map[string]*apiextensionsv1.CustomResourceDefinition{
				"captures.retina.sh":              capture,
				"retinaendpoints.retina.sh":       endpoint,
				"metricsconfigurations.retina.sh": metrics,
				"tracesconfigurations.retina.sh":  traces,
			},
		},
		{
			name:                 "disable retina endpoint",
			enableRetinaEndpoint: false,
			want: map[string]*apiextensionsv1.CustomResourceDefinition{
				"captures.retina.sh":              capture,
				"metricsconfigurations.retina.sh": metrics,
				"tracesconfigurations.retina.sh":  traces,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := kubernetesfake.NewSimpleClientset()
			apiExtensionsClient := &apiextv1fake.FakeApiextensionsV1{
				Fake: &kubeClient.Fake,
			}
			got, err := InstallOrUpdateCRDs(context.Background(), tt.enableRetinaEndpoint, apiExtensionsClient, true)
			if (err != nil) != tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Exactly(t, tt.want, got)
		})
	}
}
