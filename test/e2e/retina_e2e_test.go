//go:build e2e

package retina

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

// TestE2ERetina tests all e2e scenarios for retina
func TestE2ERetina(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	hubblechartPath := filepath.Join(rootDir, "deploy", "hubble", "manifests", "controller", "helm", "retina")

	err = jobs.LoadGenericFlags().Run()
	require.NoError(t, err, "failed to load generic flags")

	settings, err := common.LoadInfraSettings()
	require.NoError(t, err)

	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(settings.SubID, settings.ResourceGroup, settings.ClusterName, settings.Location, settings.KubeConfigFilePath, settings.CreateInfra))
	createTestInfra.Run(ctx)

	t.Cleanup(func() {
		err := jobs.DeleteTestInfra(settings.SubID, settings.ResourceGroup, settings.Location).Run()
		if err != nil {
			t.Logf("Failed to delete test infrastructure: %v", err)
		}
	})

	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t,
		jobs.InstallAndTestRetinaBasicMetrics(
			settings.KubeConfigFilePath,
			settings.ChartPath,
			common.TestPodNamespace),
	)
	basicMetricsE2E.Run(ctx)

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t,
		jobs.UpgradeAndTestRetinaAdvancedMetrics(
			settings.KubeConfigFilePath,
			settings.ChartPath,
			settings.AdvancedProfilePath,
			common.TestPodNamespace),
	)
	advanceMetricsE2E.Run(ctx)

	// Install and test Hubble basic metrics
	validatehubble := types.NewRunner(t,
		jobs.ValidateHubble(
			settings.KubeConfigFilePath,
			hubblechartPath,
			common.TestPodNamespace),
	)
	validatehubble.Run(ctx)
}
