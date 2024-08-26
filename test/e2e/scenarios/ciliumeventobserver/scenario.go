package ciliumeventobserver

import (
	"time"

	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

const (
	sleepDelay = 5 * time.Second
	TCP        = "TCP"
	UDP        = "UDP"

	PolicyDenied = "POLICY_DENIED"
)

func ValidateCiliumEventObserverDropMetric() *types.Scenario {
	name := "CiliumEventObserverDropMetric"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateDenyAllNetworkPolicy{
				NetworkPolicyNamespace: "kube-system",
				DenyAllLabelSelector:   "app=agnhost-a",
			},
		},
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      "agnhost-a",
				AgnhostNamespace: "kube-system",
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      "agnhost-a-0",
				PodNamespace: "kube-system",
				Command:      "curl -s -m 5 bing.com",
			},
			Opts: &types.StepOptions{
				ExpectError:               true,
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      "agnhost-a-0",
				PodNamespace: "kube-system",
				Command:      "curl -s -m 5 bing.com",
			},
			Opts: &types.StepOptions{
				ExpectError:               true,
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             "kube-system",
				LabelSelector:         "k8s-app=retina",
				LocalPort:             "9965",
				RemotePort:            "9965",
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=agnhost-a", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "drop-port-forward",
			},
		},
		{
			Step: &CEODropMetric{
				PortForwardedHubblePort: "9965",
				Source:                  "agnhost-a",
				Reason:                  PolicyDenied,
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
				ResourceNamespace: "kube-system",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      "agnhost-a",
				ResourceNamespace: "kube-system",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
	}
	return types.NewScenario(name, steps...)
}

// Flows Processed
func ValidateCiliumEventObserverFlowsAndTCPMetrics() *types.Scenario {
	name := "ValidateCiliumEventObserverFlowsAndTCPMetrics"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateKapingerDeployment{
				KapingerNamespace: "kube-system",
				KapingerReplicas:  "1",
			},
		},
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      "agnhost-a",
				AgnhostNamespace: "kube-system",
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      "agnhost-a-0",
				PodNamespace: "kube-system",
				Command:      "curl -s -m 5 bing.com",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      "agnhost-a-0",
				PodNamespace: "kube-system",
				Command:      "curl -s -m 5 bing.com",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             "kube-system",
				LabelSelector:         "k8s-app=retina",
				LocalPort:             "9965",
				RemotePort:            "9965",
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=agnhost-a", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "flows-port-forward",
			},
		},
		{
			Step: &CEOFlowsAndTCPMetrics{
				PortForwardedHubblePort: "9965",
				Source:                  "agnhost-a",
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "flows-port-forward",
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.NetworkPolicy),
				ResourceName:      "deny-all",
				ResourceNamespace: "kube-system",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      "agnhost-a",
				ResourceNamespace: "kube-system",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
