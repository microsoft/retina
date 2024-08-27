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
	ICMPv6     = "ICMPv6"
	HubblePort = "9965"

	PolicyDenied = "POLICY_DENIED"
	Forwarded    = "FORWARDED"
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
				LocalPort:             HubblePort,
				RemotePort:            HubblePort,
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=agnhost-a", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "drop-port-forward",
			},
		},
		{
			Step: &CEODropMetric{
				PortForwardedHubblePort: HubblePort,
				Source:                  "agnhost-a",
				Reason:                  PolicyDenied,
				Direction:               "unknown",
				Protocol:                UDP,
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			// Drop for IPv6
			Step: &CEODropMetric{
				PortForwardedHubblePort: HubblePort,
				Source:                  "agnhost-a",
				Reason:                  PolicyDenied,
				Direction:               "unknown",
				Protocol:                ICMPv6,
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
				LocalPort:             HubblePort,
				RemotePort:            HubblePort,
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=agnhost-a", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "flows-port-forward",
			},
		},
		{
			// Check IPv4
			Step: &CEOFlowsMetric{
				PortForwardedHubblePort: HubblePort,
				// Source and Destination are empty for now due to hubble enrichment bug
				// Source:      "",
				// Destination: "",
				Protocol: TCP,
				Verdict:  Forwarded,
				Type:     "Trace",
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			// Check IPv6
			Step: &CEOFlowsMetric{
				PortForwardedHubblePort: HubblePort,
				// Source and Destination are empty for now due to hubble enrichment bug
				// Source:      "",
				// Destination: "",
				Protocol: ICMPv6,
				Verdict:  Forwarded,
				Type:     "Trace",
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			// check IPv4
			Step: &CEOTCPMetric{
				PortForwardedHubblePort: HubblePort,
				// Source: "agnhost-a",
				// Destination: "",
				Flag:   "FIN",
				Family: "IPv4",
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			// check IPv6
			Step: &CEOTCPMetric{
				PortForwardedHubblePort: HubblePort,
				// Source: "agnhost-a",
				// Destination: "",
				Flag:   "FIN",
				Family: "IPv6",
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
