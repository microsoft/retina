//go:build scale

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
	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

func TestE2ERetina_Scale(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

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
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfraAZ(subID, rg, clusterName, location, kubeConfigFilePath, *createInfra))
	createTestInfra.Run(ctx)

	t.Cleanup(func() {
		if *deleteInfra {
			_ = jobs.DeleteTestInfraAZ(subID, rg, clusterName, location).Run()
		}
	})

	// Install Retina
	installRetina := types.NewRunner(t, jobs.InstallRetina(kubeConfigFilePath, chartPath))
	installRetina.Run(ctx)

	// Scale test
	opt := jobs.DefaultScaleTestOptions()
	opt.KubeconfigPath = kubeConfigFilePath
	opt.RealPodType = "kapinger"
	opt.DeleteLabels = true

	scale := types.NewRunner(t, jobs.ScaleTest(&opt))
	scale.Run(ctx)
}
