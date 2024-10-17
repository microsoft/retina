package retina

import (
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
		NumRealDeployments:            3,
		RealPodType:                   "agnhost",
		NumRealReplicas:               2,
		NumRealServices:               1,
		NumNetworkPolicies:            10,
		NumUnapliedNetworkPolicies:    10,
		NumUniqueLabelsPerPod:         2,
		NumUniqueLabelsPerDeployment:  2,
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
		DryRun:    opt.DryRun,
	}, nil)

	job.AddStep(&kubernetes.CreateNamespace{
		DryRun: opt.DryRun,
	}, nil)

	job.AddStep(&scaletest.CreateResources{
		NumKwokDeployments:           opt.NumKwokDeployments,
		NumKwokReplicas:              opt.NumKwokReplicas,
		RealPodType:                  opt.RealPodType,
		NumRealDeployments:           opt.NumRealDeployments,
		NumRealReplicas:              opt.NumRealReplicas,
		NumRealServices:              opt.NumRealServices,
		NumUniqueLabelsPerDeployment: opt.NumUniqueLabelsPerDeployment,
		DryRun:                       opt.DryRun,
	}, nil)

	job.AddStep(&scaletest.AddSharedLabelsToAllPods{
		NumSharedLabelsPerPod: opt.NumSharedLabelsPerPod,
		DryRun:                opt.DryRun,
	}, nil)

	job.AddStep(&scaletest.AddUniqueLabelsToAllPods{
		NumUniqueLabelsPerPod: opt.NumUniqueLabelsPerPod,
		DryRun:                opt.DryRun,
	}, nil)

	// Apply network policies (applied and unapplied)
	job.AddStep(&scaletest.CreateNetworkPolicies{
		NumNetworkPolicies:    opt.NumNetworkPolicies,
		NumSharedLabelsPerPod: opt.NumSharedLabelsPerPod,
		DryRun:                opt.DryRun,
	}, nil)

	job.AddStep(&kubernetes.WaitPodsReady{
		LabelSelector: "is-real=true",
		DryRun:        opt.DryRun,
	}, nil)

	job.AddStep(&scaletest.DeleteAndReAddLabels{
		DryRun:                opt.DryRun,
		DeleteLabels:          opt.DeleteLabels,
		DeleteLabelsInterval:  opt.DeleteLabelsInterval,
		DeleteLabelsTimes:     opt.DeleteLabelsTimes,
		NumSharedLabelsPerPod: opt.NumSharedLabelsPerPod,
	}, nil)

	// TODO: Add steps to get the state of the cluster

	// job.AddStep(&kubernetes.GetDeployment{})

	// job.AddStep(&kubernetes.GetDaemonSet{})

	// job.AddStep(&kubernetes.DescribePods{})

	return job
}
