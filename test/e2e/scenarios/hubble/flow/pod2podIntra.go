package flow

import (
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func intPtr(i int) *int {
	return &i
}

func ValidatePodToPodIntraNodeHubbleFlowMetric(arch string) *types.Scenario {
	var (
		podname                          = "agnhost-flow-intra"
		validPod0HubbleFlowLabelsToStack = map[string]string{
			constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podname + "-0",
			constants.HubbleDestinationLabel: "",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-stack",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validPod0HubbleFlowLablesToEndpoint = map[string]string{
			constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podname + "-0",
			constants.HubbleDestinationLabel: "",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-endpoint",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validPod1HubbleFlowLabelsToStack = map[string]string{
			constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podname + "-1",
			constants.HubbleDestinationLabel: "",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-stack",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validPod1HubbleFlowLabelsToEndpoint = map[string]string{
			constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podname + "-1",
			constants.HubbleDestinationLabel: "",
			constants.HubbleProtocolLabel:    constants.TCP,
			constants.HubbleSubtypeLabel:     "to-endpoint",
			constants.HubbleTypeLabel:        "Trace",
			constants.HubbleVerdictLabel:     "FORWARDED",
		}
		validHubbleFlowMetricsLabels = []map[string]string{
			validPod0HubbleFlowLabelsToStack,
			validPod0HubbleFlowLablesToEndpoint,
			validPod1HubbleFlowLabelsToStack,
			validPod1HubbleFlowLabelsToEndpoint,
		}
	)
	name := "Validate pod to pod intra node Hubble flow metrics - Arch: " + arch
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:        podname,
				AgnhostNamespace:   common.TestPodNamespace,
				ScheduleOnSameNode: true,
				AgnhostReplicas:    intPtr(2),
				AgnhostArch:        arch,
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
				OptionalLabelAffinity: "app=" + podname, // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID:     "hubble-flow-intra-port-forward" + arch,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &CurlPod{
				SrcPodName:      podname + "-0",
				SrcPodNamespace: common.TestPodNamespace,
				DstPodName:      podname + "-1",
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
				ValidMetrics:  validHubbleFlowMetricsLabels,
				ExpectMetric:  true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "hubble-flow-intra-port-forward" + arch,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      podname,
				ResourceNamespace: common.TestPodNamespace,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
