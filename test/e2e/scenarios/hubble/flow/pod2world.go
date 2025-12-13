package flow

import (
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func ValidatePodToWorldHubbleFlowMetric(arch string) *types.Scenario {
	var (
		podName                               = "agnhost-flow-world"
		validHubbleFlowToWorldTCPToStackLabel = map[string]string{
			constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podName + "-0",
			constants.HubbleDestinationLabel: "",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-stack",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validHubbleFlowToWorldUDPToStackLabel = map[string]string{
			constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podName + "-0",
			constants.HubbleDestinationLabel: "",
			constants.HubbleProtocolLabel:    constants.UDP,
			constants.HubbleSubtypeLabel:     "to-stack",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validHubbleFlowMetricsLabels = []map[string]string{
			validHubbleFlowToWorldTCPToStackLabel,
			validHubbleFlowToWorldUDPToStackLabel,
		}
	)
	name := "Validate pod to world Hubble flow metrics - Arch: " + arch
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      podName,
				AgnhostNamespace: common.TestPodNamespace,
				AgnhostArch:      arch,
			},
		},
		// Need this delay to guarantee that the pods will have bpf program attached
		{
			Step: &types.Sleep{
				Duration: 30 * time.Second,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.PortForward{
				LabelSelector:         "k8s-app=retina",
				LocalPort:             constants.HubbleMetricsPort,
				RemotePort:            constants.HubbleMetricsPort,
				Endpoint:              constants.MetricsEndpoint,
				OptionalLabelAffinity: "app=" + podName,
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID:     "hubble-flow-to-world-port-forward" + arch,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName + "-0",
				PodNamespace: common.TestPodNamespace,
				Command:      "curl -s -m 5 bing.com",
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &common.ValidateMetric{
				ForwardedPort: constants.HubbleMetricsPort,
				MetricName:    constants.HubbleFlowMetricName,
				ValidMetrics:  validHubbleFlowMetricsLabels,
				ExpectMetric:  true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "hubble-flow-to-world-port-forward" + arch,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      podName,
				ResourceNamespace: common.TestPodNamespace,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
