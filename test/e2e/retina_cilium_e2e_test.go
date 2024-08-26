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

var retinaCiliumLocations = []string{"eastus2", "centralus", "southcentralus", "uksouth", "centralindia", "westus2"}

// TestE2ERetinaCilium tests all e2e scenarios for retina on cilium clusters
func TestE2ERetinaCilium(t *testing.T) {
	curuser, err := user.Current()
	require.NoError(t, err)
	clusterName := curuser.Username + common.NetObsRGtag + "cil-" + strconv.FormatInt(time.Now().Unix(), 10)

	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	require.NotEmpty(t, subID)

	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		var nBig *big.Int
		nBig, err = rand.Int(rand.Reader, big.NewInt(int64(len(retinaCiliumLocations))))
		if err != nil {
			t.Fatalf("Failed to generate a secure random index: %v", err)
		}
		location = retinaCiliumLocations[nBig.Int64()]
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	chartPath := filepath.Join(rootDir, "deploy", "hubble", "manifests", "controller", "helm", "retina")
	profilePath := filepath.Join(rootDir, "test", "profiles", "advanced", "hubble", "cilium", "values.yaml")
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra with Cilium dataplane
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfraCilium(subID, clusterName, location, kubeConfigFilePath))
	createTestInfra.Run()

	// Hacky way to ensure that the test infra is deleted even if the test panics
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered in TestE2ERetinaCilium, %v", r)
		}
		_ = jobs.DeleteTestInfra(subID, clusterName, location).Run()
	}()

	advanceMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaCiliumMetrics(kubeConfigFilePath, chartPath, profilePath))
	advanceMetricsE2E.Run()
}
