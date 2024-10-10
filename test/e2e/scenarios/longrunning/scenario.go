package longrunning

import (
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func PullPProf(kubeConfigFilePath string) *types.Scenario {
	Name := "Pull PProf"
	Steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateKapingerDeployment{
				KapingerNamespace:  "kube-system",
				KapingerReplicas:   "5",
				KubeConfigFilePath: kubeConfigFilePath,
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
				DurationSeconds:       "28800", // 8 hours
				ScrapeIntervalSeconds: "60",
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
