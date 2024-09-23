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
		nBig, err = rand.Int(rand.Reader, big.NewInt(int64(len(locations))))
		if err != nil {
			t.Fatalf("Failed to generate a secure random index: %v", err)
		}
		location = locations[nBig.Int64()]
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	useExistingInfra, err := strconv.ParseBool(os.Getenv("USE_EXISTING_INFRA"))
	if err != nil {
		useExistingInfra = false
	}
	if useExistingInfra {
		clusterName = os.Getenv("CLUSTER_NAME")
		require.NotEmpty(t, clusterName)
		resGroupName := os.Getenv("RESOURCE_GROUP_NAME")
		require.NotEmpty(t, resGroupName)
		// RegisterExistingInfra
		registerExistingInfra := types.NewRunner(t, jobs.RegisterExistingInfra(subID, clusterName, resGroupName, location, kubeConfigFilePath))
		registerExistingInfra.Run()
	} else {
		// CreateTestInfra
		createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, clusterName, location, kubeConfigFilePath))
		createTestInfra.Run()
	}

	// sleep for 2 minutes to ensure that the cluster is up and running
	time.Sleep(2 * time.Minute)

	// Hacky way to ensure that the test infra is deleted even if the test panics
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered in TestE2ERetina, %v", r)
		}
		if !useExistingInfra {
			_ = jobs.DeleteTestInfra(subID, clusterName, location).Run()
		}
	}()

	// Gather benchmark results then install retina and run the performance tests
	runner := types.NewRunner(t, jobs.RunPerfTest(kubeConfigFilePath, chartPath))
	runner.Run()
}
