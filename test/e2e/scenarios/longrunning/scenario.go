package longrunning

import (
	"strconv"
	"time"

	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func PullPProf(kubeConfigFilePath string) *types.Scenario {
	Name := "Pull PProf"
	Steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateKapingerDeployment{
				KapingerNamespace:  "default",
				KapingerReplicas:   "500",
				KubeConfigFilePath: kubeConfigFilePath,
				BurstIntervalMs:    "10000", // 10 seconds
				BurstVolume:        "200",   // 500 requests every 10 seconds
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             "kube-system",
				LabelSelector:         "k8s-app=retina",
				LocalPort:             "10093",
				RemotePort:            "10093",
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=kapinger",
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "retina-port-forward",
			},
		},
		{
			Step: &kubernetes.PullPProf{
				DurationSeconds:       strconv.Itoa(int((8 * time.Hour).Seconds())), //nolint part of the test
				ScrapeIntervalSeconds: strconv.Itoa(int((1 * time.Hour).Seconds())), //nolint part of the test
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "retina-port-forward",
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.Deployment),
				ResourceName:      "kapinger",
				ResourceNamespace: "kube-system",
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}

	return types.NewScenario(Name, Steps...)
}
