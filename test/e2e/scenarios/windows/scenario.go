package windows

import (
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func ValidateWinBpfMetricScenario() *types.Scenario {
	name := "Validate Windows BPF Basic and Advanced Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &ValidateWinBpfMetric{
				KubeConfigFilePath:        "./test.pem",
				RetinaDaemonSetNamespace:  common.KubeSystemNamespace,
				RetinaDaemonSetName:       "retina-agent-win",
				EbpfXdpDeamonSetNamespace: "install-ebpf-xdp",
				EbpfXdpDeamonSetName:      "install-ebpf-xdp",
				NonHpcAppNamespace:        "default",
				NonHpcAppName:             "non-hpc",
				NonHpcPodName:             "non-hpc-pod",
			},
		},
	}
	return types.NewScenario(name, steps...)
}

func ValidateWindowsBasicMetric() *types.Scenario {
	name := "Windows Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &ValidateHNSMetric{
				KubeConfigFilePath:       "./test.pem",
				RetinaDaemonSetNamespace: common.KubeSystemNamespace,
				RetinaDaemonSetName:      "retina-agent-win",
			},
		},
	}
	return types.NewScenario(name, steps...)
}
