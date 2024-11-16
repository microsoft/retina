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
	"github.com/microsoft/retina/test/e2e/framework/helpers"
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
func TestE2ERetinaAZ(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

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
	profilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfraAZ(subID, rg, clusterName, location, kubeConfigFilePath, *createInfra))
	createTestInfra.Run(ctx)

	t.Cleanup(func() {
		if *deleteInfra {
			_ = jobs.DeleteTestInfraAZ(subID, rg, clusterName, location).Run()
		}
	})

	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath, "azure", common.TestPodNamespace))
	basicMetricsE2E.Run(ctx)

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t, jobs.UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, profilePath, "azure", common.TestPodNamespace))
	advanceMetricsE2E.Run(ctx)
}

func TestE2ERetinaAWS(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

	curuser, err := user.Current()
	require.NoError(t, err)

	clusterName := curuser.Username + common.NetObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)

	accID := os.Getenv("AWS_ACCOUNT_ID")
	require.NotEmpty(t, accID)

	region := os.Getenv("AWS_REGION")

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	profilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfraAWS(accID, clusterName, region, kubeConfigFilePath))
	createTestInfra.Run(ctx)

	t.Cleanup(func() {
		if *deleteInfra {
			_ = jobs.DeleteTestInfraAWS(accID, clusterName, region).Run()
		}
	})

	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath, "aws", common.TestPodNamespace))
	basicMetricsE2E.Run(ctx)

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t, jobs.UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, profilePath, "aws", common.TestPodNamespace))
	advanceMetricsE2E.Run(ctx)
}
