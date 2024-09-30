package windows

import (
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func ValidateWindowsBasicMetric() *types.Scenario {
	name := "Windows Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &ValidateHNSMetric{
				KubeConfigFilePath: "./test.pem",
				RetinaPodNamespace: "kube-system",
			},
		},
	}
	return types.NewScenario(name, steps...)
}
