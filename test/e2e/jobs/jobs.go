package retina

import (
	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/scenarios/retina/dns"
	"github.com/microsoft/retina/test/e2e/scenarios/retina/drop"
	tcp "github.com/microsoft/retina/test/e2e/scenarios/retina/tcp"
)

func CreateTestInfra(subID, clusterName, location string) *types.Job {
	job := types.NewJob("Create e2e test infrastructure")

	job.AddStep(&azure.CreateResourceGroup{
		SubscriptionID:    subID,
		ResourceGroupName: clusterName,
		Location:          location,
	}, nil)

	job.AddStep(&azure.CreateVNet{
		VnetName:         "testvnet",
		VnetAddressSpace: "10.0.0.0/9",
	}, nil)

	job.AddStep(&azure.CreateSubnet{
		SubnetName:         "testsubnet",
		SubnetAddressSpace: "10.0.0.0/12",
	}, nil)

	job.AddStep(&azure.CreateNPMCluster{
		ClusterName:  clusterName,
		PodCidr:      "10.128.0.0/9",
		DNSServiceIP: "192.168.0.10",
		ServiceCidr:  "192.168.0.0/28",
	}, nil)

	job.AddStep(&azure.GetAKSKubeConfig{
		KubeConfigFilePath: "./test.pem",
	}, nil)

	job.AddStep(&generic.LoadFlags{
		TagEnv:            generic.DefaultTagEnv,
		ImageNamespaceEnv: generic.DefaultImageNamespace,
		ImageRegistryEnv:  generic.DefaultImageRegistry,
	}, nil)

	return job
}

func DeleteTestInfra(subID, clusterName, location string) *types.Job {
	job := types.NewJob("Delete e2e test infrastructure")

	job.AddStep(&azure.DeleteResourceGroup{
		SubscriptionID:    subID,
		ResourceGroupName: clusterName,
		Location:          location,
	}, nil)

	return job
}

func InstallAndTestRetinaWithBasicMetrics() *types.Job {
	job := types.NewJob("Install and test Retina with basic metrics")

	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:   "kube-system",
		ReleaseName: "retina",
		ChartPath:   "../../../deploy/manifests/controller/helm/retina/",
	}, nil)

	job.AddScenario(drop.ValidateDropMetric())

	job.AddScenario(tcp.ValidateTCPMetrics())

	job.AddScenario(dns.ValidateBasicDNSMetrics())

	return job
}

func UpgradeAndTestRetinaWithAdvancedMetrics() *types.Job {
	job := types.NewJob("Install and test Retina with advanced metrics")
	// enable advanced metrics
	job.AddStep(&kubernetes.UpgradeRetinaHelmChart{
		Namespace:   "kube-system",
		ReleaseName: "retina",
		ChartPath:   "../../../deploy/manifests/controller/helm/retina/",
		ValuesFile:  "../../profiles/localctx/values.yaml",
	}, nil)

	job.AddScenario(dns.ValidateAdvanceDNSMetrics())

	return job
}
