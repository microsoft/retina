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

var locations = []string{"eastus2", "centralus", "southcentralus", "uksouth", "centralindia", "westus2"}

// TestE2ERetina tests all e2e scenarios for retina
func TestE2ERetinaAZ(t *testing.T) {
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
	profilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfraAZ(subID, clusterName, location, kubeConfigFilePath))
	createTestInfra.Run()

	// Hacky way to ensure that the test infra is deleted even if the test panics
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered in TestE2ERetina, %v", r)
		}
		_ = jobs.DeleteTestInfraAZ(subID, clusterName, location).Run()
	}()

	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath, "azure"))
	basicMetricsE2E.Run()

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t, jobs.UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, profilePath, "azure"))
	advanceMetricsE2E.Run()
}

func TestE2ERetinaAWS(t *testing.T) {
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
	createTestInfra.Run()

	// Finalizer to clean up test infra
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered in TestE2ERetina, %v", r)
		}
		_ = jobs.DeleteTestInfraAWS(accID, clusterName, region).Run()
	}()

	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath, "aws"))
	basicMetricsE2E.Run()

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t, jobs.UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, profilePath, "aws"))
	advanceMetricsE2E.Run()
}
