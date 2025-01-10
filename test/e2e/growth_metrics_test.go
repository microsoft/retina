package retina

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/metrics"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/stretchr/testify/require"
)

func GetKubeconfig(clusterName, subscriptionId, resourceGroup, kubeConfigFilePath string) *types.Job {
	job := types.NewJob("Get kubeconfig")
	job.AddStep(&azure.GetAKSKubeConfig{
		ClusterName:        clusterName,
		SubscriptionID:     subscriptionId,
		ResourceGroupName:  resourceGroup,
		Location:           "why?",
		KubeConfigFilePath: kubeConfigFilePath,
	}, nil)
	return job
}

func GrowthTest(additionalTelemetryProperty map[string]string, kubeConfigFilePath string) *types.Job {
	job := types.NewJob("Growth Test")
	labelAffinity := "app.kubernetes.io/instance=prometheus-kube-prometheus-prometheus"
	portForwardId := "port-forward"
	metricsStepId := "metrics"

	job.AddStep(&kubernetes.PortForward{
		KubeConfigFilePath:    kubeConfigFilePath,
		Namespace:             common.KubeSystemNamespace,
		LabelSelector:         "app.kubernetes.io/instance=prometheus-kube-prometheus-prometheus",
		LocalPort:             strconv.Itoa(common.PrometheusPort),
		RemotePort:            strconv.Itoa(common.PrometheusPort),
		Endpoint:              "metrics",
		OptionalLabelAffinity: labelAffinity,
	},
		&types.StepOptions{
			SkipSavingParametersToJob: true,
			RunInBackgroundWithID:     portForwardId,
		})

	job.AddStep(&metrics.QueryAndPublish{
		Endpoint:                    "http://localhost:" + strconv.Itoa(common.PrometheusPort),
		Query:                       "scrape_samples_scraped{job=\"retina-pods\"}",
		AdditionalTelemetryProperty: additionalTelemetryProperty,
	},
		&types.StepOptions{
			SkipSavingParametersToJob: true,
			RunInBackgroundWithID:     metricsStepId,
		})

	job.AddStep(&types.Sleep{
		Duration: 60 * time.Second,
	}, nil)

	job.AddStep(
		&types.Stop{
			BackgroundID: metricsStepId,
		}, nil)

	job.AddStep(
		&types.Stop{
			BackgroundID: portForwardId,
		}, nil)
	return job
}

func Test_GrowthOfMetrics(t *testing.T) {
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

	RetinaVersion := os.Getenv(generic.DefaultTagEnv)
	require.NotEmpty(t, RetinaVersion)

	additionalTelemetryProperty := map[string]string{}
	additionalTelemetryProperty["retinaVersion"] = RetinaVersion
	additionalTelemetryProperty["clusterName"] = clusterName
	additionalTelemetryProperty["resourceGroup"] = rg

	cwd, err := os.Getwd()
	require.NoError(t, err)

	rootDir := filepath.Dir(filepath.Dir(cwd))
	kubeConfigFilePath := filepath.Join(rootDir, "test", "e2e", "test.pem")

	getKubeconfig := types.NewRunner(t, GetKubeconfig(clusterName, subID, rg, kubeConfigFilePath))
	getKubeconfig.Run(ctx)

	growth := types.NewRunner(t, GrowthTest(additionalTelemetryProperty, kubeConfigFilePath))
	growth.Run(ctx)
}
