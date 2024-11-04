package retina

import (
	"crypto/rand"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

// This test creates a new k8s cluster runs some network performance tests
// saves the data as benchmark information and then installs retina and runs the performance tests
// to compare the results and publishes a json with regression information.
func TestE2EPerfRetina(t *testing.T) {
	curuser, err := user.Current()
	require.NoError(t, err)

	clusterName := curuser.Username + common.NetObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)

	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	require.NotEmpty(t, subID, "AZURE_SUBSCRIPTION_ID environment variable must be set")

	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		var nBig *big.Int
		nBig, err = rand.Int(rand.Reader, big.NewInt(int64(len(common.AzureLocations))))
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

	appInsightsKey := os.Getenv("AZURE_APP_INSIGHTS_KEY")
	require.NotEmpty(t, appInsightsKey, "AZURE_APP_INSIGHTS_KEY environment variable must be set")

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, rg, clusterName, location, kubeConfigFilePath, true))
	createTestInfra.Run()

	t.Cleanup(func() {
		err := jobs.DeleteTestInfra(subID, rg, clusterName, location).Run()
		if err != nil {
			t.Logf("Failed to delete test infrastructure: %v", err)
		}
	})

	// Gather benchmark results then install retina and run the performance tests
	runner := types.NewRunner(t, jobs.RunPerfTest(kubeConfigFilePath, chartPath))
	runner.Run()
}
