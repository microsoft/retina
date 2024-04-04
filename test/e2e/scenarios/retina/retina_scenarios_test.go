// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package retina

import (
	"os"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/scenarios/retina/dns"
	"github.com/microsoft/retina/test/e2e/scenarios/retina/drop"
	lat "github.com/microsoft/retina/test/e2e/scenarios/retina/latency"
	tcp "github.com/microsoft/retina/test/e2e/scenarios/retina/tcp"
)

const (
	// netObsRGtag is used to tag resources created by this test suite
	netObsRGtag      = "-e2e-netobs-"
	basicMode        = "basic"
	localContextMode = "localContext"
)

// Test against AKS cluster with NPM enabled,
// create a pod with a deny all network policy and validate
// that the drop metrics are present in the prometheus endpoint
func TestE2ERetinaMetrics(t *testing.T) {
	job := types.NewJob("Validate that drop metrics are present in the prometheus endpoint")
	runner := types.NewRunner(t, job)
	defer runner.Run()

	curuser, _ := user.Current()

	testName := curuser.Username + netObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)
	sub := os.Getenv("AZURE_SUBSCRIPTION_ID")

	job.AddStep(&azure.CreateResourceGroup{
		SubscriptionID:    sub,
		ResourceGroupName: testName,
		Location:          "eastus",
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
		ClusterName:  testName,
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

	// todo: enable mutating images in helm chart
	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:   "kube-system",
		ReleaseName: "retina",
		ChartPath:   "../../../../deploy/manifests/controller/helm/retina/",
	}, &types.StepOptions{
		SkipSavingParamatersToJob: true,
	})

	job.AddScenario(drop.ValidateDropMetric())

	// todo: handle multiple scenarios back to back
	job.AddScenario(tcp.ValidateTCPMetrics())

	// check advanced metrics
	job.AddScenario(dns.ValidateDNSMetric())

	// enable advanced metrics
	job.AddStep(&kubernetes.UpgradeRetinaHelmChart{
		Namespace:   "kube-system",
		ReleaseName: "retina",
		ChartPath:   "../../../../deploy/manifests/controller/helm/retina/",
	}, &types.StepOptions{
		SkipSavingParamatersToJob: true,
	})

	// Check api server latency metrics
	job.AddScenario(lat.ValidateLatencyMetric())

	job.AddStep(&azure.DeleteResourceGroup{}, nil)
}
