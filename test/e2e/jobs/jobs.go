package retina

import (
	"fmt"
	"time"

	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/scenarios/dns"
	"github.com/microsoft/retina/test/e2e/scenarios/drop"
	"github.com/microsoft/retina/test/e2e/scenarios/latency"
	"github.com/microsoft/retina/test/e2e/scenarios/perf"
	tcp "github.com/microsoft/retina/test/e2e/scenarios/tcp"
	"github.com/microsoft/retina/test/e2e/scenarios/windows"
)

func CreateTestInfra(subID, clusterName, location, kubeConfigFilePath string, createInfra bool) *types.Job {
	job := types.NewJob("Create e2e test infrastructure")

	if createInfra {
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
			KubeConfigFilePath: kubeConfigFilePath,
		}, nil)

	} else {
		job.AddStep(&azure.GetAKSKubeConfig{
			KubeConfigFilePath: kubeConfigFilePath,
			ClusterName:        clusterName,
			SubscriptionID:     subID,
			ResourceGroupName:  clusterName,
			Location:           location,
		}, nil)
	}

	job.AddStep(&generic.LoadFlags{
		TagEnv:            generic.DefaultTagEnv,
		ImageNamespaceEnv: generic.DefaultImageNamespace,
		ImageRegistryEnv:  generic.DefaultImageRegistry,
	}, nil)

	return job
}

func RegisterExistingInfra(subID, clusterName, resGroupName, location, kubeConfigFilePath string) *types.Job {
	job := types.NewJob("Register existing e2e test infrastructure")

	job.AddStep(&azure.GetAKSKubeConfig{
		SubscriptionID:     subID,
		ResourceGroupName:  resGroupName,
		ClusterName:        clusterName,
		Location:           location,
		KubeConfigFilePath: kubeConfigFilePath,
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

func InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath string) *types.Job {
	job := types.NewJob("Install and test Retina with basic metrics")

	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:          "kube-system",
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
	}, nil)

	job.AddScenario(drop.ValidateDropMetric())

	job.AddScenario(tcp.ValidateTCPMetrics())

	job.AddScenario(windows.ValidateWindowsBasicMetric())

	dnsScenarios := []struct {
		name string
		req  *dns.RequestValidationParams
		resp *dns.ResponseValidationParams
	}{
		{
			name: "Validate basic DNS request and response metrics for a valid domain",
			req: &dns.RequestValidationParams{
				NumResponse: "0",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				Command:     "nslookup kubernetes.default",
				ExpectError: false,
			},
			resp: &dns.ResponseValidationParams{
				NumResponse: "1",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				ReturnCode:  "No Error",
				Response:    "10.0.0.1",
			},
		},
		{
			name: "Validate basic DNS request and response metrics for a non-existent domain",
			req: &dns.RequestValidationParams{
				NumResponse: "0",
				Query:       "some.non.existent.domain.",
				QueryType:   "A",
				Command:     "nslookup some.non.existent.domain",
				ExpectError: true,
			},
			resp: &dns.ResponseValidationParams{
				NumResponse: "0",
				Query:       "some.non.existent.domain.",
				QueryType:   "A",
				Response:    dns.EmptyResponse, // hacky way to bypass the framework for now
				ReturnCode:  "Non-Existent Domain",
			},
		},
	}

	for _, scenario := range dnsScenarios {
		job.AddScenario(dns.ValidateBasicDNSMetrics(scenario.name, scenario.req, scenario.resp))
	}

	job.AddStep(&kubernetes.EnsureStableCluster{
		PodNamespace:  "kube-system",
		LabelSelector: "k8s-app=retina",
	}, nil)

	return job
}

func UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, valuesFilePath string) *types.Job {
	job := types.NewJob("Upgrade and test Retina with advanced metrics")
	// enable advanced metrics
	job.AddStep(&kubernetes.UpgradeRetinaHelmChart{
		Namespace:          "kube-system",
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
		ValuesFile:         valuesFilePath,
	}, nil)

	dnsScenarios := []struct {
		name string
		req  *dns.RequestValidationParams
		resp *dns.ResponseValidationParams
	}{
		{
			name: "Validate advanced DNS request and response metrics for a valid domain",
			req: &dns.RequestValidationParams{
				NumResponse: "0",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				Command:     "nslookup kubernetes.default",
				ExpectError: false,
			},
			resp: &dns.ResponseValidationParams{
				NumResponse: "1",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				ReturnCode:  "NOERROR",
				Response:    "10.0.0.1",
			},
		},
		{
			name: "Validate advanced DNS request and response metrics for a non-existent domain",
			req: &dns.RequestValidationParams{
				NumResponse: "0",
				Query:       "some.non.existent.domain.",
				QueryType:   "A",
				Command:     "nslookup some.non.existent.domain.",
				ExpectError: true,
			},
			resp: &dns.ResponseValidationParams{
				NumResponse: "0",
				Query:       "some.non.existent.domain.",
				QueryType:   "A",
				Response:    dns.EmptyResponse, // hacky way to bypass the framework for now
				ReturnCode:  "NXDOMAIN",
			},
		},
	}

	for _, scenario := range dnsScenarios {
		job.AddScenario(dns.ValidateAdvancedDNSMetrics(scenario.name, scenario.req, scenario.resp, kubeConfigFilePath))
	}

	job.AddScenario(latency.ValidateLatencyMetric())

	job.AddStep(&kubernetes.EnsureStableCluster{
		PodNamespace:  "kube-system",
		LabelSelector: "k8s-app=retina",
	}, nil)

	return job
}

func RunPerfTest(kubeConfigFilePath string, chartPath string) *types.Job {
	job := types.NewJob("Run performance tests")

	benchmarkFile := fmt.Sprintf("netperf-benchmark-%s.json", time.Now().Format("20060102150405"))
	resultFile := fmt.Sprintf("netperf-result-%s.json", time.Now().Format("20060102150405"))

	job.AddStep(&perf.GetNetworkPerformanceMeasures{
		KubeConfigFilePath: kubeConfigFilePath,
		ResultTag:          "no-retina",
		JsonOutputFile:     benchmarkFile,
	}, &types.StepOptions{
		SkipSavingParametersToJob: true,
	})

	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:          "kube-system",
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
	}, nil)

	job.AddStep(&perf.GetNetworkPerformanceMeasures{
		KubeConfigFilePath: kubeConfigFilePath,
		ResultTag:          "retina",
		JsonOutputFile:     resultFile,
	}, &types.StepOptions{
		SkipSavingParametersToJob: true,
	})

	job.AddStep(&perf.GetNetworkRegressionResults{
		BaseResultsFile: benchmarkFile,
		NewResultsFile:  resultFile,
	}, &types.StepOptions{
		SkipSavingParametersToJob: true,
	})

	return job
}
