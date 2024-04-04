// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
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
				OptionalLabelAffinity: " ",
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "latency-port-forward",
			},
		},
		{
			Step: &ValidateAPIServerLatencyMetric{
				PortForwardedRetinaPort: "10093",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
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
