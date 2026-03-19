//go:build e2e

package retina

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/framework/helpers"
	"github.com/microsoft/retina/test/e2ev3/infra"
	"github.com/microsoft/retina/test/e2ev3/workflows/advancedmetrics"
	"github.com/microsoft/retina/test/e2ev3/workflows/basicmetrics"
	"github.com/microsoft/retina/test/e2ev3/workflows/capture"
	"github.com/microsoft/retina/test/e2ev3/workflows/flags"
	"github.com/microsoft/retina/test/e2ev3/workflows/hubblemetrics"
	"github.com/stretchr/testify/require"
)

// TestE2ERetina tests all e2e scenarios for retina using the go-workflow framework.
func TestE2ERetina(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories.
	rootDir := filepath.Dir(filepath.Dir(cwd))

	// Load generic flags (image registry, namespace, tag).
	flagsWF := flags.LoadGenericFlags()
	require.NoError(t, flagsWF.Do(ctx), "failed to load generic flags")

	if *common.KubeConfig == "" {
		*common.KubeConfig = infra.CreateAzureTempK8sInfra(ctx, t, rootDir)
	}

	kubeConfigPath := common.KubeConfigFilePath(rootDir)

	// Install and test Retina basic metrics.
	t.Run("BasicMetrics", func(t *testing.T) {
		wf := basicmetrics.InstallAndTestRetinaBasicMetrics(
			kubeConfigPath,
			common.RetinaChartPath(rootDir),
			common.TestPodNamespace,
		)
		require.NoError(t, wf.Do(ctx), "basic metrics workflow failed")
	})

	// Upgrade and test Retina with advanced metrics.
	t.Run("AdvancedMetrics", func(t *testing.T) {
		wf := advancedmetrics.UpgradeAndTestRetinaAdvancedMetrics(
			kubeConfigPath,
			common.RetinaChartPath(rootDir),
			common.RetinaAdvancedProfilePath(rootDir),
			common.TestPodNamespace,
		)
		require.NoError(t, wf.Do(ctx), "advanced metrics workflow failed")
	})

	// Install and test Hubble metrics.
	t.Run("HubbleMetrics", func(t *testing.T) {
		wf := hubblemetrics.InstallAndTestHubbleMetrics(
			kubeConfigPath,
			common.HubbleChartPath(rootDir),
		)
		require.NoError(t, wf.Do(ctx), "hubble metrics workflow failed")
	})

	// Install Retina basic and test captures.
	t.Run("Capture", func(t *testing.T) {
		wf := capture.ValidateCapture(
			kubeConfigPath,
			"default",
		)
		require.NoError(t, wf.Do(ctx), "capture workflow failed")
	})
}
