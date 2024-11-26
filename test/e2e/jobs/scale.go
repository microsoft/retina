package retina

import (
	"os"
	"time"

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
		MaxRealPodsPerNode:            100,
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
		DeleteLabels:                  false,
		DeleteLabelsInterval:          60 * time.Second,
		DeleteLabelsTimes:             1,
		DeleteNetworkPolicies:         false,
		DeleteNetworkPoliciesInterval: 60 * time.Second,
		DeleteNetworkPoliciesTimes:    1,
		LabelsToGetMetrics:            map[string]string{},
		AdditionalTelemetryProperty:   map[string]string{},
	}
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

	job.AddStep(&scaletest.GetAndPublishMetrics{
		Labels:                      opt.LabelsToGetMetrics,
		AdditionalTelemetryProperty: opt.AdditionalTelemetryProperty,
		OutputFilePath:                  os.Getenv("OUTPUT_FILEPATH"),
	}, &types.StepOptions{
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

	job.AddStep(&kubernetes.DeleteNamespace{}, nil)

	return job
}
