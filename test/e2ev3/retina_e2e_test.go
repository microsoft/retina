//go:build e2e

package retina

import (
	"crypto/rand"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/infra"
	"github.com/microsoft/retina/test/e2ev3/pkg/azure"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	"github.com/microsoft/retina/test/e2ev3/workflows/advancedmetrics"
	"github.com/microsoft/retina/test/e2ev3/workflows/basicmetrics"
	"github.com/microsoft/retina/test/e2ev3/workflows/capture"
	"github.com/microsoft/retina/test/e2ev3/workflows/hubblemetrics"
	"github.com/stretchr/testify/require"
)

// TestE2ERetina tests all e2e scenarios for retina using the go-workflow framework.
func TestE2ERetina(t *testing.T) {
	ctx, cancel := common.Context(t)
	defer cancel()

	cfg, err := config.LoadE2EConfig()
	require.NoError(t, err, "failed to load e2e config")

	cwd, err := os.Getwd()
	require.NoError(t, err)

	rootDir := filepath.Dir(filepath.Dir(cwd))

	// Resolve infrastructure config from environment.
	kubeConfigFilePath := common.KubeConfigFilePath(rootDir)
	if *common.KubeConfig == "" {
		infraCfg := resolveInfraConfig(t, &cfg.Azure)
		infra.CreateAzureTempK8sInfra(ctx, t, infraCfg, kubeConfigFilePath, *common.CreateInfra, *common.DeleteInfra)
	}

	// Install and test Retina basic metrics.
	t.Run("BasicMetrics", func(t *testing.T) {
		wf := basicmetrics.InstallAndTestRetinaBasicMetrics(
			kubeConfigFilePath,
			common.RetinaChartPath(rootDir),
			common.TestPodNamespace,
			&cfg.Image,
			&cfg.Helm,
		)
		require.NoError(t, wf.Do(ctx), "basic metrics workflow failed")
	})

	// Upgrade and test Retina with advanced metrics.
	t.Run("AdvancedMetrics", func(t *testing.T) {
		wf := advancedmetrics.UpgradeAndTestRetinaAdvancedMetrics(
			kubeConfigFilePath,
			common.RetinaChartPath(rootDir),
			common.RetinaAdvancedProfilePath(rootDir),
			common.TestPodNamespace,
			&cfg.Helm,
		)
		require.NoError(t, wf.Do(ctx), "advanced metrics workflow failed")
	})

	// Install and test Hubble metrics.
	t.Run("HubbleMetrics", func(t *testing.T) {
		wf := hubblemetrics.InstallAndTestHubbleMetrics(
			kubeConfigFilePath,
			common.HubbleChartPath(rootDir),
			&cfg.Image,
			&cfg.Helm,
		)
		require.NoError(t, wf.Do(ctx), "hubble metrics workflow failed")
	})

	// Experimental: test additional basic metrics (forward, conntrack, TCP stats, network stats, node connectivity).
	t.Run("BasicMetricsExperimental", func(t *testing.T) {
		wf := basicmetrics.InstallAndTestRetinaBasicMetricsExperimental(
			kubeConfigFilePath,
			common.RetinaChartPath(rootDir),
			common.TestPodNamespace,
			&cfg.Image,
			&cfg.Helm,
		)
		require.NoError(t, wf.Do(ctx), "basic metrics experimental workflow failed")
	})

	// Experimental: test additional advanced metrics (drop, forward, TCP flags/retrans, API server latency).
	t.Run("AdvancedMetricsExperimental", func(t *testing.T) {
		wf := advancedmetrics.UpgradeAndTestRetinaAdvancedMetricsExperimental(
			kubeConfigFilePath,
			common.RetinaChartPath(rootDir),
			common.RetinaAdvancedProfilePath(rootDir),
			common.TestPodNamespace,
			&cfg.Helm,
		)
		require.NoError(t, wf.Do(ctx), "advanced metrics experimental workflow failed")
	})

	// Install Retina basic and test captures.
	t.Run("Capture", func(t *testing.T) {
		wf := capture.ValidateCapture(
			kubeConfigFilePath,
			"default",
			&cfg.Image,
		)
		require.NoError(t, wf.Do(ctx), "capture workflow failed")
	})
}

// resolveInfraConfig builds the Azure infrastructure config, using viper-loaded values
// and falling back to random location / generated cluster name when not set.
func resolveInfraConfig(t *testing.T, azureCfg *config.AzureConfig) *azure.InfraConfig {
	t.Helper()

	subID := azureCfg.SubscriptionID
	require.NotEmpty(t, subID, "AZURE_SUBSCRIPTION_ID must be set")

	location := azureCfg.Location
	if location == "" {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(common.AzureLocations))))
		require.NoError(t, err)
		location = common.AzureLocations[nBig.Int64()]
	}

	clusterName := common.ClusterNameForE2ETest(t, azureCfg.ClusterName)

	rg := azureCfg.ResourceGroup
	if rg == "" {
		rg = clusterName
	}

	return azure.DefaultE2EInfraConfig(subID, rg, location, clusterName)
}
