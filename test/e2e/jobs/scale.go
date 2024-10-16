package retina

import (
	"time"

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

	return job
}
