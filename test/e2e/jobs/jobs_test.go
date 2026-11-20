package retina

import (
	"testing"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/scenarios/windows"
	"github.com/stretchr/testify/require"
)

func TestBasicMetricsChecksWindowsImageBeforeScenarios(t *testing.T) {
	originalYear := *common.WindowsYear
	t.Cleanup(func() { *common.WindowsYear = originalYear })
	for _, year := range []string{"2022", "2025"} {
		t.Run(year, func(t *testing.T) {
			*common.WindowsYear = year
			job := InstallAndTestRetinaBasicMetrics("test-kubeconfig", "test-chart", "test-namespace")
			require.IsType(t, &kubernetes.InstallHelmChart{}, job.Steps[0].Step)
			check, ok := job.Steps[1].Step.(*windows.ValidateImage)
			require.True(t, ok, "image verification must immediately follow installation")
			require.Equal(t, year, check.Year)
			require.Equal(t, "test-kubeconfig", check.KubeConfigFilePath)
			require.Nil(t, job.Scenarios[job.Steps[1]], "image verification must precede metric scenarios")
		})
	}
}

func TestEventWriterImage(t *testing.T) {
	t.Setenv(generic.DefaultImageRegistry, "registry.example.com")
	t.Setenv(generic.DefaultImageNamespace, "candidate/retina")
	t.Setenv(generic.DefaultTagEnv, "v1.2.9")

	for _, tc := range []struct {
		name     string
		override string
		want     string
	}{
		{
			name: "default",
			want: "registry.example.com/candidate/retina/test/e2e-test-event-writer:v1.2.9",
		},
		{
			name:     "independent tag",
			override: "registry.example.com/helpers/event-writer:stable",
			want:     "registry.example.com/helpers/event-writer:stable",
		},
		{
			name:     "digest",
			override: "registry.example.com/helpers/event-writer@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:     "registry.example.com/helpers/event-writer@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EVENT_WRITER_IMAGE", tc.override)
			require.Equal(t, tc.want, eventWriterImage())
		})
	}
}
