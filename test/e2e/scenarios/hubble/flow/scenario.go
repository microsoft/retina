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

func ValidateFlowMetric() *types.Scenario {
	name := "Flow Metrics"
	agnhostName := "agnhost-flow"
	podName := agnhostName + "-0"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateKapingerDeployment{
				KapingerNamespace: common.KubeSystemNamespace,
				KapingerReplicas:  "1",
			},
		},
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      agnhostName,
				AgnhostNamespace: common.TestPodNamespace,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: common.TestPodNamespace,
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
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: common.TestPodNamespace,
				Command:      "curl -s -m 5 bing.com",
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
				RunInBackgroundWithID:     "hubble-flow-port-forward",
				SkipSavingParametersToJob: true,
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
				BackgroundID: "hubble-flow-port-forward",
			},
		},
	}

	return types.NewScenario(name, steps...)
}
