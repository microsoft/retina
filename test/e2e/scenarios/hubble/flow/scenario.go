package flow

import (
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

const (
	sleepDelay = 5 * time.Second
)

func ValidateFlowMetric(namespace, arch string) *types.Scenario {
	id := "hubble-flow-port-forward-" + arch
	name := "Hubble Flow Metrics - Arch: " + arch
	agnhostName := "agnhost-flow"
	podName := agnhostName + "-0"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      agnhostName,
				AgnhostNamespace: namespace,
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
				Namespace:             common.KubeSystemNamespace,
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=" + agnhostName, // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID:     id,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: namespace,
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
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: id,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      agnhostName,
				ResourceNamespace: namespace,
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}

	return types.NewScenario(name, steps...)
}
