package retina

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/scenarios/capture"
	"github.com/microsoft/retina/test/e2e/scenarios/dns"
	"github.com/microsoft/retina/test/e2e/scenarios/drop"
	hubble_dns "github.com/microsoft/retina/test/e2e/scenarios/hubble/dns"
	hubble_drop "github.com/microsoft/retina/test/e2e/scenarios/hubble/drop"
	hubble_flow "github.com/microsoft/retina/test/e2e/scenarios/hubble/flow"
	hubble_service "github.com/microsoft/retina/test/e2e/scenarios/hubble/service"
	hubble_tcp "github.com/microsoft/retina/test/e2e/scenarios/hubble/tcp"
	"github.com/microsoft/retina/test/e2e/scenarios/latency"
	tcp "github.com/microsoft/retina/test/e2e/scenarios/tcp"
	"github.com/microsoft/retina/test/e2e/scenarios/windows"
)

const IPPrefix = "serviceTaggedIp"

func CreateTestInfra(subID, rg, clusterName, location, kubeConfigFilePath string, createInfra bool) *types.Job {
	job := types.NewJob("Create e2e test infrastructure")

	publicIPID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/publicIPAddresses", subID, clusterName)
	publicIPv4FullName := fmt.Sprintf("%s/%s-%s-v4", publicIPID, IPPrefix, clusterName)
	publicIPv6FullName := fmt.Sprintf("%s/%s-%s-v6", publicIPID, IPPrefix, clusterName)

	if createInfra {
		job.AddStep(&azure.CreateResourceGroup{
			SubscriptionID:    subID,
			ResourceGroupName: rg,
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

		job.AddStep(&azure.CreatePublicIP{
			ClusterName: clusterName,
			IPVersion:   string(armnetwork.IPVersionIPv4),
			IPPrefix:    IPPrefix,
		}, &types.StepOptions{
			SkipSavingParametersToJob: true,
		})

		job.AddStep(&azure.CreatePublicIP{
			ClusterName: clusterName,
			IPVersion:   string(armnetwork.IPVersionIPv6),
			IPPrefix:    IPPrefix,
		}, &types.StepOptions{
			SkipSavingParametersToJob: true,
		})

		job.AddStep(&azure.CreateNPMCluster{
			ClusterName:  clusterName,
			PodCidr:      "10.128.0.0/9",
			DNSServiceIP: "192.168.0.10",
			ServiceCidr:  "192.168.0.0/28",
			PublicIPs: []string{
				publicIPv4FullName,
				publicIPv6FullName,
			},
		}, nil)

		job.AddStep(&azure.GetAKSKubeConfig{
			KubeConfigFilePath: kubeConfigFilePath,
		}, nil)

	} else {
		job.AddStep(&azure.GetAKSKubeConfig{
			KubeConfigFilePath: kubeConfigFilePath,
			ClusterName:        clusterName,
			SubscriptionID:     subID,
			ResourceGroupName:  rg,
			Location:           location,
		}, nil)
	}

	return job
}

func DeleteTestInfra(subID, rg, location string, deleteInfra bool) *types.Job {
	job := types.NewJob("Delete e2e test infrastructure")

	if deleteInfra {
		job.AddStep(&azure.DeleteResourceGroup{
			SubscriptionID:    subID,
			ResourceGroupName: rg,
			Location:          location,
		}, nil)
	}

	return job
}

func InstallRetina(kubeConfigFilePath, chartPath string, enableHeartBeat bool) *types.Job {
	job := types.NewJob("Install and test Retina with basic metrics")

	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
		EnableHeartbeat:    enableHeartBeat,
	}, nil)

	return job
}

func UninstallRetina(kubeConfigFilePath, chartPath string) *types.Job {
	job := types.NewJob("Uninstall Retina")

	job.AddStep(&kubernetes.UninstallHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
	}, nil)

	return job
}

func InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath string, testPodNamespace string) *types.Job {
	job := types.NewJob("Install and test Retina with basic metrics")

	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
	}, nil)

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

	for _, arch := range common.Architectures {
		job.AddScenario(drop.ValidateDropMetric(testPodNamespace, arch))
		job.AddScenario(tcp.ValidateTCPMetrics(testPodNamespace, arch))

		for _, scenario := range dnsScenarios {
			name := scenario.name + " - Arch: " + arch
			job.AddScenario(dns.ValidateBasicDNSMetrics(name, scenario.req, scenario.resp, testPodNamespace, arch))
		}

		job.AddScenario(windows.ValidateWindowsBasicMetric())
	}

	job.AddStep(&kubernetes.EnsureStableComponent{
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		IgnoreContainerRestart: false,
	}, nil)

	return job
}

func UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, valuesFilePath string, testPodNamespace string) *types.Job {
	job := types.NewJob("Upgrade and test Retina with advanced metrics")
	// enable advanced metrics
	job.AddStep(&kubernetes.UpgradeRetinaHelmChart{
		Namespace:          common.KubeSystemNamespace,
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

	for _, arch := range common.Architectures {
		for _, scenario := range dnsScenarios {
			name := scenario.name + " - Arch: " + arch
			job.AddScenario(dns.ValidateAdvancedDNSMetrics(name, scenario.req, scenario.resp, kubeConfigFilePath, testPodNamespace, arch))
		}
	}

	job.AddScenario(latency.ValidateLatencyMetric(testPodNamespace))

	job.AddStep(&kubernetes.EnsureStableComponent{
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		IgnoreContainerRestart: false,
	}, nil)

	return job
}

func InstallAndTestHubbleMetrics(kubeConfigFilePath, chartPath string) *types.Job {
	job := types.NewJob("Validate Hubble")

	job.AddStep(&kubernetes.InstallHubbleHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
	}, nil)

	hubbleScenarios := []*types.Scenario{
		hubble_service.ValidateHubbleRelayService(),
		hubble_service.ValidateHubbleUIService(kubeConfigFilePath),
	}

	for _, arch := range common.Architectures {
		hubbleScenarios = append(hubbleScenarios,
			hubble_dns.ValidateDNSMetric(arch),
			hubble_flow.ValidatePodToPodIntraNodeHubbleFlowMetric(arch),
			hubble_flow.ValidatePodToPodInterNodeHubbleFlowMetric(arch),
			hubble_flow.ValidatePodToWorldHubbleFlowMetric(arch),
			hubble_drop.ValidateDropMetric(arch),
			hubble_tcp.ValidateTCPMetric(arch),
		)
	}

	for _, scenario := range hubbleScenarios {
		job.AddScenario(scenario)
	}

	job.AddStep(&kubernetes.EnsureStableComponent{
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		IgnoreContainerRestart: false,
	}, nil)

	return job
}

func ValidateCapture(kubeConfigFilePath, testPodNamespace string) *types.Job {
	job := types.NewJob("Validate Capture")

	job.AddScenario(capture.ValidateCapture(
		kubeConfigFilePath,
		testPodNamespace))

	return job
}

func LoadGenericFlags() *types.Job {
	job := types.NewJob("Loading Generic Flags to env")

	job.AddStep(&generic.LoadFlags{
		TagEnv:            generic.DefaultTagEnv,
		ImageNamespaceEnv: generic.DefaultImageNamespace,
		ImageRegistryEnv:  generic.DefaultImageRegistry,
	}, nil)

	return job
}
