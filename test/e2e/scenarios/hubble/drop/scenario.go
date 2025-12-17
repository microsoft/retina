package drop

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

func ValidateDropMetric(arch string) *types.Scenario {
	name := "Drop Metrics - Arch: " + arch
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateDenyAllNetworkPolicy{
				NetworkPolicyNamespace: common.TestPodNamespace,
				DenyAllLabelSelector:   "app=" + agnhostName,
			},
		},
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      agnhostName,
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
				LocalPort:             constants.RetinaMetricsPort,
				RemotePort:            constants.RetinaMetricsPort,
				Endpoint:              constants.MetricsEndpoint,
				OptionalLabelAffinity: "app=" + agnhostName, // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID:     "retina-drop-port-forward" + arch,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.PortForward{
				LabelSelector:         "k8s-app=retina",
				LocalPort:             constants.HubbleMetricsPort,
				RemotePort:            constants.HubbleMetricsPort,
				Endpoint:              constants.MetricsEndpoint,
				OptionalLabelAffinity: "app=" + agnhostName, // port forward hubble metrics to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID:     "hubble-drop-port-forward" + arch,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: common.TestPodNamespace,
				Command:      "curl -s -m 5 bing.com",
			},
			Opts: &types.StepOptions{
				ExpectError:               true,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &common.ValidateMetric{
				ForwardedPort: constants.RetinaMetricsPort,
				MetricName:    constants.RetinaDropMetricName,
				ValidMetrics:  []map[string]string{validRetinaDropMetricLabels},
				ExpectMetric:  true,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &common.ValidateMetric{
				ForwardedPort: constants.HubbleMetricsPort,
				MetricName:    constants.HubbleDropMetricName,
				ValidMetrics:  []map[string]string{validHubbleDropMetricLabels},
				ExpectMetric:  true,
				PartialMatch:  true,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "hubble-drop-port-forward" + arch,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "retina-drop-port-forward" + arch,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.NetworkPolicy),
				ResourceName:      "deny-all",
				ResourceNamespace: common.TestPodNamespace,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      agnhostName,
				ResourceNamespace: common.TestPodNamespace,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
