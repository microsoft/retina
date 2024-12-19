//go:build scale

package retina

import (
	"os"
	"path/filepath"
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

	rg := os.Getenv("AZURE_RESOURCE_GROUP")
	if rg == "" {
		// Use the cluster name as the resource group name by default.
		rg = clusterName
	}

	kubeConfigFilePath := filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Scale test parameters
	opt := jobs.DefaultScaleTestOptions()
	opt.KubeconfigPath = kubeConfigFilePath

	// TODO: Get Retina Version from cluster or change ENV VAR
	RetinaVersion := os.Getenv(generic.DefaultTagEnv)
	require.NotEmpty(t, RetinaVersion)
	opt.AdditionalTelemetryProperty["retinaVersion"] = RetinaVersion
	opt.AdditionalTelemetryProperty["clusterName"] = clusterName

	// AppInsightsKey is required for telemetry
	require.NotEmpty(t, os.Getenv(common.AzureAppInsightsKeyEnv))

	// Agent label
	opt.LabelsToGetMetrics = map[string]string{"k8s-app": "retina"}

	fqdn, err := azure.GetFqdnFn(subID, rg, clusterName)
	require.NoError(t, err)
	opt.AdditionalTelemetryProperty["clusterFqdn"] = fqdn

	scale := types.NewRunner(t, jobs.ScaleTest(&opt))
	scale.Run(ctx)
}
