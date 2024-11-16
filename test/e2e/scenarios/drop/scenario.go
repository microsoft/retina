package drop

import (
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

const (
	sleepDelay = 5 * time.Second
	TCP        = "TCP"
	UDP        = "UDP"

	IPTableRuleDrop = "IPTABLE_RULE_DROP"
)

func ValidateDropMetric(namespace string) *types.Scenario {
	name := "Drop Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateDenyAllNetworkPolicy{
				NetworkPolicyNamespace: namespace,
				DenyAllLabelSelector:   "app=agnhost-drop",
			},
		},
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostNamespace: namespace,
				AgnhostName:      "agnhost-drop",
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
				PodName:      "agnhost-drop-0",
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
				PodName:      "agnhost-drop-0",
				Command:      "curl -s -m 5 bing.com",
			},
			Opts: &types.StepOptions{
				ExpectError:               true,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             common.KubeSystemNamespace,
				LabelSelector:         "k8s-app=retina",
				LocalPort:             "10093",
				RemotePort:            "10093",
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=agnhost-drop", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "drop-port-forward",
			},
		},
		{
			Step: &ValidateRetinaDropMetric{
				PortForwardedRetinaPort: "10093",
				Source:                  "agnhost-drop",
				Reason:                  IPTableRuleDrop,
				Direction:               "unknown",
				Protocol:                UDP,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "drop-port-forward",
			},
		},

		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.NetworkPolicy),
				ResourceName:      "deny-all",
				ResourceNamespace: namespace,
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceNamespace: namespace,
				ResourceName:      "agnhost-drop",
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
