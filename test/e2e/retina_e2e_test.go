package retina

import (
	"crypto/rand"
	"flag"
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

var (
	locations   = []string{"eastus2", "centralus", "southcentralus", "uksouth", "centralindia", "westus2"}
	createInfra = flag.Bool("create-infra", true, "create a Resource group, vNET and AKS cluster for testing")
	deleteInfra = flag.Bool("delete-infra", true, "delete a Resource group, vNET and AKS cluster for testing")
)

// TestE2ERetina tests all e2e scenarios for retina
func TestE2ERetina(t *testing.T) {
	curuser, err := user.Current()
	require.NoError(t, err)
	flag.Parse()

	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		username := curuser.Username
		// Truncate the username to 8 characters
		if len(username) > 8 {
			username = username[:8]
			t.Logf("Username is too long, truncating to 8 characters: %s", username)
		}
		clusterName = username + common.NetObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)
		t.Logf("CLUSTER_NAME is not set, generating a random cluster name: %s", clusterName)
	}

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
	createTestInfra.Run()

	// Hacky way to ensure that the test infra is deleted even if the test panics
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered in TestE2ERetina, %v", r)
		}
		if *deleteInfra {
			_ = jobs.DeleteTestInfra(subID, rg, clusterName, location).Run()
		}
	}()

	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath, common.TestPodNamespace))
	basicMetricsE2E.Run()

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t, jobs.UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, profilePath, common.TestPodNamespace))
	advanceMetricsE2E.Run()

	// Install and test Retina basic metrics
	validatehubble := types.NewRunner(t, jobs.ValidateHubble(kubeConfigFilePath, hubblechartPath, common.TestPodNamespace))
	validatehubble.Run()
}
