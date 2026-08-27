//go:build scale

package retina

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/params"
	"github.com/microsoft/retina/test/e2e/framework/scaletest"
	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

func TestE2ERetina_Scale(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

	clusterName := common.ScaleTestInfra.GetClusterName()
	subID := common.ScaleTestInfra.GetSubscriptionID()
	require.NotEmpty(t, subID)
	location := common.ScaleTestInfra.GetLocation()
	rg := common.ScaleTestInfra.GetResourceGroup()
	linuxNodes, err := strconv.ParseInt(common.ScaleTestInfra.GetLinuxNodes(), 10, 32)
	require.NoError(t, err, "LINUX_NODES must be an integer within int32 range")
	windowsNodes, err := strconv.ParseInt(common.ScaleTestInfra.GetWindowsNodes(), 10, 32)
	require.NoError(t, err, "WINDOWS_NODES must be an integer within int32 range")

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	err = jobs.LoadGenericFlags().Run()
	require.NoError(t, err, "failed to load generic flags")

	// Scale test parameters
	opt := jobs.DefaultScaleTestOptions()
	opt.KubeconfigPath = common.KubeConfigFilePath(rootDir)

	NumDeployments := params.NumDeployments
	NumReplicas := params.NumReplicas
	NumNetworkPolicies := params.NumNetworkPolicies
	CleanUp := params.CleanUp

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
		opt.CleanUp, err = strconv.ParseBool(CleanUp)
		require.NoError(t, err)
	}

	opt.NumLinuxNodes = int(linuxNodes)
	opt.NumWindowsNodes = int(windowsNodes)
	opt.NumLinuxDeployments, opt.NumWindowsDeployments, err = scaletest.SplitCountByNodes(opt.NumRealDeployments, opt.NumLinuxNodes, opt.NumWindowsNodes)
	require.NoError(t, err)
	opt.NumLinuxServices, opt.NumWindowsServices, err = scaletest.SplitCountByNodes(opt.NumRealServices, opt.NumLinuxNodes, opt.NumWindowsNodes)
	require.NoError(t, err)

	RetinaVersion := os.Getenv(generic.DefaultTagEnv)
	require.NotEmpty(t, RetinaVersion)
	opt.AdditionalTelemetryProperty["retinaVersion"] = RetinaVersion
	opt.AdditionalTelemetryProperty["clusterName"] = clusterName

	// AppInsightsKey is required for telemetry
	require.NotEmpty(t, os.Getenv(common.AzureAppInsightsKeyEnv))

	opt.LabelsToGetMetrics = map[string]string{"k8s-app": "retina"}

	// CreateTestInfra
	infra := types.NewRunner(t, jobs.GetScaleTestInfra(subID, rg, clusterName, location, common.KubeConfigFilePath(rootDir), int32(linuxNodes), int32(windowsNodes), *common.CreateInfra))

	t.Cleanup(func() {
		_ = jobs.DeleteTestInfra(subID, rg, location, *common.DeleteInfra).Run()
	})

	infra.Run(ctx)

	fqdn, err := azure.GetFqdnFn(subID, rg, clusterName)
	require.NoError(t, err)
	opt.AdditionalTelemetryProperty["clusterFqdn"] = fqdn

	// Install Retina
	installRetina := types.NewRunner(t, jobs.InstallRetinaForScale(common.KubeConfigFilePath(rootDir), common.RetinaChartPath(rootDir), true))
	installRetina.Run(ctx)

	scale := types.NewRunner(t, jobs.ScaleTest(&opt))
	scale.Run(ctx)
}
