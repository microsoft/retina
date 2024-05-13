package flow

import (
	"time"

	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

const (
	sleepDelay = 5 * time.Second
	TCP        = "TCP"
	UDP        = "UDP"

	IPTableRuleDrop = "IPTABLE_RULE_DROP"
)

func ValidateTCPMetrics() *types.Scenario {
	Name := "Flow Metrics"
	Steps := []*types.StepWrapper{
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
				LabelSelector:         "k8s-app=retina",
				Namespace:             "kube-system",
				LocalPort:             "10093",
				RemotePort:            "10093",
				OptionalLabelAffinity: "app=agnhost-a", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
				RunInBackgroundWithID:     "drop-flow-forward",
			},
		},
		{
			Step: &ValidateRetinaTCPStateMetric{
				PortForwardedRetinaPort: "10093",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &ValidateRetinaTCPConnectionRemoteMetric{
				PortForwardedRetinaPort: "10093",
			}, Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "drop-flow-forward",
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

	return types.NewScenario(Name, Steps...)
}
