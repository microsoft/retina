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
	name := "Drop Metrics - Arch: " + arch
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateDenyAllNetworkPolicy{
				NetworkPolicyNamespace: namespace,
				DenyAllLabelSelector:   "app=" + agnhostName,
			},
		},
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostNamespace: namespace,
				AgnhostName:      agnhostName,
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
				Namespace:             common.KubeSystemNamespace,
				LabelSelector:         "k8s-app=retina",
				LocalPort:             constants.RetinaMetricsPort,
				RemotePort:            constants.RetinaMetricsPort,
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=" + agnhostName, // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID:     "drop-port-forward" + arch,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &ValidateRetinaDropMetric{
				PortForwardedRetinaPort: constants.RetinaMetricsPort,
				Source:                  agnhostName,
				Reason:                  constants.IPTableRuleDrop,
				Direction:               validRetinaDropMetricLabels[constants.RetinaDirectionLabel],
				Protocol:                constants.UDP,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "drop-port-forward" + arch,
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
				ResourceName:      agnhostName,
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
