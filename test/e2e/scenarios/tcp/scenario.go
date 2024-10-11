package flow

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

func ValidateTCPMetrics(namespace string) *types.Scenario {
	Name := "Flow Metrics"
	Steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateKapingerDeployment{
				KapingerNamespace: namespace,
				KapingerReplicas:  "1",
				BurstIntervalMs:   "10000", // 10 seconds
				BurstVolume:       "10",    // 500 requests every 10 seconds
			},
		},
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      "agnhost-a",
				AgnhostNamespace: namespace,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      "agnhost-a-0",
				PodNamespace: namespace,
				Command:      "curl -s -m 5 bing.com",
			}, Opts: &types.StepOptions{
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
				PodName:      "agnhost-a-0",
				PodNamespace: namespace,
				Command:      "curl -s -m 5 bing.com",
			}, Opts: &types.StepOptions{
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
				OptionalLabelAffinity: "app=agnhost-a", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
				RunInBackgroundWithID:     "drop-flow-forward",
			},
		},
		{
			Step: &ValidateRetinaTCPStateMetric{
				PortForwardedRetinaPort: "10093",
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &ValidateRetinaTCPConnectionRemoteMetric{
				PortForwardedRetinaPort: "10093",
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
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
				ResourceNamespace: namespace,
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}

	return types.NewScenario(Name, Steps...)
}
