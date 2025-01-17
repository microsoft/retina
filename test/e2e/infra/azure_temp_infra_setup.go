package infra

import (
	"context"
	"crypto/rand"
	"math/big"
	"os"
	"testing"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

func CreateAzureTempK8sInfra(t *testing.T, rootDir string, ctx context.Context) string {
	kubeConfigFilePath := common.KubeConfigFilePath(rootDir)
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

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, rg, clusterName, location, kubeConfigFilePath, *common.CreateInfra))
	createTestInfra.Run(ctx)

	t.Cleanup(func() {
		err := jobs.DeleteTestInfra(subID, rg, location, *common.DeleteInfra).Run()
		if err != nil {
			t.Logf("Failed to delete test infrastructure: %v", err)
		}
	})
	return kubeConfigFilePath
}
