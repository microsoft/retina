package latency

import (
	"time"

	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

const sleepDelay = 5 * time.Second

func ValidateLatencyMetric() *types.Scenario {
	name := "Latency Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             "kube-system",
				LabelSelector:         "k8s-app=retina",
				LocalPort:             "10093",
				RemotePort:            "10093",
				Endpoint:              "metrics",
				OptionalLabelAffinity: "k8s-app=retina",
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
				RunInBackgroundWithID:     "latency-port-forward",
			},
		},
		{
			Step: &ValidateAPIServerLatencyMetric{},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "latency-port-forward",
			},
		},
	}
	return types.NewScenario(name, steps...)
}
