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

func ValidateDropMetric(namespace, arch string) *types.Scenario {
	id := "hubble-drop-port-forward-" + arch
	name := "Hubble Drop Metrics - Arch: " + arch
	agnhostName := "agnhost-drop"
	podName := agnhostName + "-0"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateDenyAllNetworkPolicy{
				NetworkPolicyNamespace: namespace,
				DenyAllLabelSelector:   "app=agnhost-drop",
			},
		},
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
			Step: &kubernetes.ExecInPod{
				PodNamespace: namespace,
				PodName:      podName,
				Command:      "curl -s -m 5 bing.com",
			},
			Opts: &types.StepOptions{
				ExpectError:               true,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodNamespace: namespace,
				PodName:      podName,
				Command:      "curl -s -m 5 bing.com",
			},
			Opts: &types.StepOptions{
				ExpectError:               true,
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
				PodNamespace: namespace,
				PodName:      podName,
				Command:      "curl -s -m 5 bing.com",
			},
			Opts: &types.StepOptions{
				ExpectError:               true,
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
				OptionalLabelAffinity: "app=" + agnhostName, // port forward hubble metrics to a pod on a node that also has this pod with this label, assuming same namespace
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
				ExpectError:               true,
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
				MetricName:    constants.HubbleDropMetricName,
				ValidMetrics:  []map[string]string{validHubbleDropMetricLabels},
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
				ResourceType:      kubernetes.TypeString(kubernetes.NetworkPolicy),
				ResourceName:      "deny-all",
				ResourceNamespace: namespace,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceNamespace: namespace,
				ResourceName:      agnhostName,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
