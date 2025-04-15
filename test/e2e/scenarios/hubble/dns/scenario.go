package dns

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

func ValidateDNSMetric(namespace, arch string) *types.Scenario {
	id := "hubble-dns-port-forward-" + arch
	name := "Hubble DNS Metrics - Arch: " + arch
	agnhostName := "agnhost-dns"
	podName := agnhostName + "-0"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      agnhostName,
				AgnhostNamespace: namespace,
				AgnhostArch:      arch,
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
				RunInBackgroundWithID: id,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: namespace,
				Command:      "nslookup -type=a one.one.one.one",
			},
			Opts: &types.StepOptions{
				ExpectError:               false,
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
				MetricName:    constants.HubbleDNSQueryMetricName,
				ValidMetrics:  []map[string]string{validDNSQueryMetricLabels},
				ExpectMetric:  true,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &common.ValidateMetric{
				ForwardedPort: constants.HubbleMetricsPort,
				MetricName:    constants.HubbleDNSResponseMetricName,
				ValidMetrics:  []map[string]string{validDNSResponseMetricLabels},
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
	}
	return types.NewScenario(name, steps...)
}
