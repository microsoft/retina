//go:build perf

package retina

import (
	"crypto/rand"
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

// This test creates a new k8s cluster runs some network performance tests
// saves the data as benchmark information and then installs retina and runs the performance tests
// to compare the results and publishes a json with regression information.
func TestE2EPerfRetina(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

	clusterName := common.ClusterNameForE2ETest(t)

	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	require.NotEmpty(t, subID, "AZURE_SUBSCRIPTION_ID environment variable must be set")

	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(common.AzureLocations))))
		if err != nil {
			t.Fatal("Failed to generate a secure random index", err)
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

	appInsightsKey := os.Getenv(common.AzureAppInsightsKeyEnv)
	if appInsightsKey == "" {
		t.Log("No app insights key provided, results will be saved locally at ./ as `netperf-benchmark-*`, `netperf-result-*`, and `netperf-regression-*`")
	}

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, rg, clusterName, location, kubeConfigFilePath, true))
	createTestInfra.Run(ctx)

	t.Cleanup(func() {
		err := jobs.DeleteTestInfra(subID, rg, clusterName, location).Run()
		if err != nil {
			t.Logf("Failed to delete test infrastructure: %v", err)
		}
	})

	// Gather benchmark results then install retina and run the performance tests
	runner := types.NewRunner(t, jobs.RunPerfTest(kubeConfigFilePath, chartPath))
	runner.Run(ctx)
}
