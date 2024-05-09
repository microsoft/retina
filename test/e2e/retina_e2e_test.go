package retina

import (
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
func TestE2ERetina(t *testing.T) {
	curuser, err := user.Current()
	require.NoError(t, err)

	clusterName := curuser.Username + common.NetObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)

	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	require.NotEmpty(t, subID)

	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		location = "eastus"
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "manifests", "controller", "helm", "retina")
	profilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, clusterName, location, kubeConfigFilePath))
	createTestInfra.Run()

	// Hacky way to ensure that the test infra is deleted even if the test panics
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		t.Logf("Recovered in TestE2ERetina, %v", r)
	// 	}
	// 	_ = jobs.DeleteTestInfra(subID, clusterName, location).Run()
	// }()

	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaWithBasicMetrics(kubeConfigFilePath, chartPath))
	basicMetricsE2E.Run()

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t, jobs.UpgradeAndTestRetinaWithAdvancedMetrics(kubeConfigFilePath, chartPath, profilePath))
	advanceMetricsE2E.Run()
}
