package flow

import (
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func ValidatePodToPodInterNodeHubbleFlowMetric(arch string) *types.Scenario {
	var (
		podnameSrc                           = "agnhost-flow-inter-src"
		podnameDst                           = "agnhost-flow-inter-dst"
		validHubbleFlowMetricSrcToStackLabel = map[string]string{
			constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podnameSrc + "-0",
			constants.HubbleDestinationLabel: "",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-stack",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validHubbleFlowMetricSrcToEndpointLabel = map[string]string{
			constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podnameDst + "-0",
			constants.HubbleDestinationLabel: "",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-endpoint",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validHubbleFlowMetricDstToStackLabel = map[string]string{
			constants.HubbleSourceLabel:      "",
			constants.HubbleDestinationLabel: common.TestPodNamespace + "/" + podnameSrc + "-0",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-stack",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validHubbleFlowMetricDstToEndpointLabel = map[string]string{
			constants.HubbleSourceLabel:      "",
			constants.HubbleDestinationLabel: common.TestPodNamespace + "/" + podnameDst + "-0",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-endpoint",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validHubbleFlowMetricsSrcLabels = []map[string]string{
			validHubbleFlowMetricSrcToStackLabel,
			validHubbleFlowMetricSrcToEndpointLabel,
		}
		validHubbleFlowMetricsDstLabels = []map[string]string{
			validHubbleFlowMetricDstToStackLabel,
			validHubbleFlowMetricDstToEndpointLabel,
		}
	)
	name := "Validate pod to pod inter node Hubble flow metrics - Arch: " + arch
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      podnameSrc,
				AgnhostNamespace: common.TestPodNamespace,
				AgnhostArch:      arch,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      podnameDst,
				AgnhostNamespace: common.TestPodNamespace,
				AgnhostArch:      arch,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
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
				OptionalLabelAffinity: "app=" + podnameSrc,
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID:     "hubble-src-flow-port-forward" + arch,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.PortForward{
				LabelSelector:         "k8s-app=retina",
				LocalPort:             "9966",
				RemotePort:            constants.HubbleMetricsPort,
				Endpoint:              constants.MetricsEndpoint,
				OptionalLabelAffinity: "app=" + podnameDst,
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID:     "hubble-dst-flow-port-forward" + arch,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &CurlPod{
				SrcPodName:      podnameSrc + "-0",
				SrcPodNamespace: common.TestPodNamespace,
				DstPodName:      podnameDst + "-0",
				DstPodNamespace: common.TestPodNamespace,
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
				ValidMetrics:  validHubbleFlowMetricsSrcLabels,
				ExpectMetric:  true,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &common.ValidateMetric{
				ForwardedPort: "9966",
				MetricName:    constants.HubbleFlowMetricName,
				ValidMetrics:  validHubbleFlowMetricsDstLabels,
				ExpectMetric:  true,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "hubble-src-flow-port-forward" + arch,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "hubble-dst-flow-port-forward" + arch,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      podnameSrc,
				ResourceNamespace: common.TestPodNamespace,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      podnameDst,
				ResourceNamespace: common.TestPodNamespace,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
