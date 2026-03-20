// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	flow "github.com/Azure/go-workflow"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addNetworkStatsScenario(kubeConfigFilePath string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	validateIPStats := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_ip_connection_stats",
		ValidMetrics:  []map[string]string{{}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateUDPStats := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_udp_connection_stats",
		ValidMetrics:  []map[string]string{{}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateIfaceStats := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_interface_stats",
		ValidMetrics:  []map[string]string{{}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath,
		},
		Steps: []flow.Steper{validateIPStats, validateUDPStats, validateIfaceStats},
	}

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			Retry(utils.RetryWithBackoff),
	)
	return wf
}
