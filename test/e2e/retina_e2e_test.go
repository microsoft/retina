//go:build e2e

package retina

import (
	"crypto/rand"
	"flag"
	"math/big"
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

	flag.Parse()

	// Truncate the username to 8 characters
	clusterName := common.ClusterNameForE2ETest(t)

	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	require.NotEmpty(t, subID)

	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(common.AzureLocations))))
		if err != nil {
			t.Fatalf("Failed to generate a secure random index: %v", err)
		}
		location = common.AzureLocations[nBig.Int64()]
	}

	rg := os.Getenv("AZURE_RESOURCE_GROUP")
	if rg == "" {
		// Use the cluster name as the resource group name by default.
		rg = clusterName
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	hubblechartPath := filepath.Join(rootDir, "deploy", "hubble", "manifests", "controller", "helm", "retina")
	profilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, rg, clusterName, location, kubeConfigFilePath, *createInfra))
	createTestInfra.Run(ctx)

	t.Cleanup(func() {
		if *deleteInfra {
			_ = jobs.DeleteTestInfra(subID, rg, clusterName, location).Run()
		}
	})

	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath, common.TestPodNamespace))
	basicMetricsE2E.Run(ctx)

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t, jobs.UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, profilePath, common.TestPodNamespace))
	advanceMetricsE2E.Run(ctx)

	// Install and test Hubble basic metrics
	validatehubble := types.NewRunner(t, jobs.ValidateHubble(kubeConfigFilePath, hubblechartPath, common.TestPodNamespace))
	validatehubble.Run(ctx)
}
