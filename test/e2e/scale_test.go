//go:build scale

package retina

import (
	"crypto/rand"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

func TestE2ERetina_Scale(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

	clusterName := common.ClusterNameForE2ETest(t)

	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	require.NotEmpty(t, subID)

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

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	jobs.LoadGenericFlags().Run()

	// Scale test parameters
	opt := jobs.DefaultScaleTestOptions()
	opt.KubeconfigPath = common.KubeConfigFilePath(rootDir)

	NumDeployments := os.Getenv("NUM_DEPLOYMENTS")
	NumReplicas := os.Getenv("NUM_REPLICAS")
	NumNetworkPolicies := os.Getenv("NUM_NET_POL")
	CleanUp := os.Getenv("CLEANUP")

	if NumDeployments != "" {
		opt.NumRealDeployments, err = strconv.Atoi(NumDeployments)
		opt.NumRealServices = opt.NumRealDeployments
		require.NoError(t, err)
	}
	if NumReplicas != "" {
		opt.NumRealReplicas, err = strconv.Atoi(NumReplicas)
		require.NoError(t, err)
	}
	if NumNetworkPolicies != "" {
		opt.NumNetworkPolicies, err = strconv.Atoi(NumNetworkPolicies)
		require.NoError(t, err)
	}
	if CleanUp != "" {
		opt.DeleteLabels, err = strconv.ParseBool(CleanUp)
		require.NoError(t, err)
	}

	RetinaVersion := os.Getenv(generic.DefaultTagEnv)
	require.NotEmpty(t, RetinaVersion)
	opt.AdditionalTelemetryProperty["retinaVersion"] = RetinaVersion
	opt.AdditionalTelemetryProperty["clusterName"] = clusterName

	// AppInsightsKey is required for telemetry
	require.NotEmpty(t, os.Getenv(common.AzureAppInsightsKeyEnv))

	opt.LabelsToGetMetrics = map[string]string{"k8s-app": "retina"}

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, rg, clusterName, location, common.KubeConfigFilePath(rootDir), *common.CreateInfra))
	createTestInfra.Run(ctx)

	t.Cleanup(func() {
		_ = jobs.DeleteTestInfra(subID, rg, location, *common.DeleteInfra).Run()
	})

	fqdn, err := azure.GetFqdnFn(subID, rg, clusterName)
	require.NoError(t, err)
	opt.AdditionalTelemetryProperty["clusterFqdn"] = fqdn

	// Install Retina
	installRetina := types.NewRunner(t, jobs.InstallRetina(common.KubeConfigFilePath(rootDir), common.RetinaChartPath(rootDir)))
	installRetina.Run(ctx)

	t.Cleanup(func() {
		_ = jobs.UninstallRetina(common.KubeConfigFilePath(rootDir), common.RetinaChartPath(rootDir)).Run()
	})

	scale := types.NewRunner(t, jobs.ScaleTest(&opt))
	scale.Run(ctx)
}
