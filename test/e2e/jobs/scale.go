package retina

import (
	"os"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/scaletest"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func DefaultScaleTestOptions() scaletest.Options {
	return scaletest.Options{
		Namespace:                     "scale-test",
		MaxKwokPodsPerNode:            0,
		NumKwokDeployments:            0,
		NumKwokReplicas:               0,
		MaxRealPodsPerNode:            250,
		NumRealDeployments:            1000,
		RealPodType:                   "kapinger",
		NumRealReplicas:               40,
		NumRealServices:               1000,
		NumNetworkPolicies:            10,
		NumUnapliedNetworkPolicies:    10,
		NumUniqueLabelsPerPod:         0,
		NumUniqueLabelsPerDeployment:  1,
		NumSharedLabelsPerPod:         3,
		KubeconfigPath:                "",
		RestartNpmPods:                false,
		SleepAfterCreation:            0,
		DeleteKwokPods:                false,
		DeletePodsInterval:            60 * time.Second,
		DeleteRealPods:                false,
		DeletePodsTimes:               1,
		DeleteLabels:                  true,
		DeleteLabelsInterval:          60 * time.Second,
		DeleteLabelsTimes:             1,
		DeleteNetworkPolicies:         false,
		DeleteNetworkPoliciesInterval: 60 * time.Second,
		DeleteNetworkPoliciesTimes:    1,
		LabelsToGetMetrics:            map[string]string{},
		AdditionalTelemetryProperty:   map[string]string{},
		CleanUp:                       true,
	}
}

func GetScaleTestInfra(subID, rg, clusterName, location, kubeConfigFilePath string, nodes int32, createInfra bool) *types.Job {
	job := types.NewJob("Get scale test infrastructure")

	if createInfra {
		job.AddStep(&azure.CreateResourceGroup{
			SubscriptionID:    subID,
			ResourceGroupName: rg,
			Location:          location,
		}, nil)

		job.AddStep((&azure.CreateCluster{
			ClusterName: clusterName,
			Nodes:       nodes,
		}).
			SetPodCidr("100.64.0.0/10").
			SetVMSize("Standard_D4_v3").
			SetNetworkPluginMode("overlay"), nil)

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

	job.AddStep(&kubernetes.LabelNodes{
		Labels: map[string]string{"scale-test": "true"},
	}, nil)

	job.AddStep(&generic.LoadFlags{
		TagEnv:            generic.DefaultTagEnv,
		ImageNamespaceEnv: generic.DefaultImageNamespace,
		ImageRegistryEnv:  generic.DefaultImageRegistry,
	}, nil)

	return job
}

func ScaleTest(opt *scaletest.Options) *types.Job {
	job := types.NewJob("Scale Test")

	job.AddStep(&scaletest.ValidateAndPrintOptions{
		Options: opt,
	}, nil)

	job.AddStep(&scaletest.ValidateNumOfNodes{
		KubeConfigFilePath: opt.KubeconfigPath,
		Label:              map[string]string{"scale-test": "true"},
		NumNodesRequired: (opt.NumRealDeployments*opt.NumRealReplicas +
			opt.MaxRealPodsPerNode - 1) / opt.MaxRealPodsPerNode,
	}, nil)

	job.AddStep(&kubernetes.DeleteNamespace{
		Namespace: opt.Namespace,
	}, nil)

	job.AddStep(&kubernetes.CreateNamespace{}, nil)

	// There's a known limitation on leaving empty fields in Steps.
	// Set methods are used to set private fields and keep environment variables accessed within jobs, rather then spread through steps.
	job.AddStep((&scaletest.GetAndPublishMetrics{
		Labels:                      opt.LabelsToGetMetrics,
		AdditionalTelemetryProperty: opt.AdditionalTelemetryProperty,
	}).
		SetOutputFilePath(os.Getenv(common.OutputFilePathEnv)).
		SetAppInsightsKey(os.Getenv(common.AzureAppInsightsKeyEnv)),
		&types.StepOptions{
			SkipSavingParametersToJob: true,
			RunInBackgroundWithID:     "get-metrics",
		})

	job.AddStep(&scaletest.CreateResources{
		NumKwokDeployments:           opt.NumKwokDeployments,
		NumKwokReplicas:              opt.NumKwokReplicas,
		RealPodType:                  opt.RealPodType,
		NumRealDeployments:           opt.NumRealDeployments,
		NumRealReplicas:              opt.NumRealReplicas,
		NumRealServices:              opt.NumRealServices,
		NumUniqueLabelsPerDeployment: opt.NumUniqueLabelsPerDeployment,
	}, nil)

	job.AddStep(&scaletest.AddSharedLabelsToAllPods{
		NumSharedLabelsPerPod: opt.NumSharedLabelsPerPod,
	}, nil)

	job.AddStep(&scaletest.AddUniqueLabelsToAllPods{
		NumUniqueLabelsPerPod: opt.NumUniqueLabelsPerPod,
	}, nil)

	// Apply network policies (applied and unapplied)
	job.AddStep(&scaletest.CreateNetworkPolicies{
		NumNetworkPolicies:    opt.NumNetworkPolicies,
		NumSharedLabelsPerPod: opt.NumSharedLabelsPerPod,
	}, nil)

	job.AddStep(&kubernetes.WaitPodsReady{
		LabelSelector: "is-real=true",
	}, nil)

	job.AddStep(&scaletest.DeleteAndReAddLabels{
		DeleteLabels:          opt.DeleteLabels,
		DeleteLabelsInterval:  opt.DeleteLabelsInterval,
		DeleteLabelsTimes:     opt.DeleteLabelsTimes,
		NumSharedLabelsPerPod: opt.NumSharedLabelsPerPod,
	}, nil)

	job.AddStep(&types.Stop{
		BackgroundID: "get-metrics",
	}, nil)

	if opt.CleanUp {
		job.AddStep(&kubernetes.DeleteNamespace{}, nil)
	}

	return job
}
