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

// TestE2ERetina tests all e2e scenarios for retina
func TestPerfRetina(t *testing.T) {
	curuser, err := user.Current()
	require.NoError(t, err)

	clusterName := curuser.Username + common.NetObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)

	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	require.NotEmpty(t, subID)

	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		var nBig *big.Int
		nBig, err = rand.Int(rand.Reader, big.NewInt(int64(len(common.AzureLocations))))
		if err != nil {
			t.Fatalf("Failed to generate a secure random index: %v", err)
		}
		location = common.AzureLocations[nBig.Int64()]
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, clusterName, location, kubeConfigFilePath))
	createTestInfra.Run()

	// Hacky way to ensure that the test infra is deleted even if the test panics
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered in TestE2ERetina, %v", r)
		}
		err := jobs.DeleteTestInfra(subID, clusterName, location).Run()
		if err != nil {
			t.Logf("Failed to delete test infrastructure: %v", err)
		}
	}()

	// Gather benchmark results then install retina and run the performance tests
	runner := types.NewRunner(t, jobs.RunPerfTest(kubeConfigFilePath, chartPath))
	runner.Run()
}
