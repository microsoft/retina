// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addNetworkStatsScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath string) flow.Steper {
	validateIPStats := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_ip_connection_stats",
		ValidMetrics:  []map[string]string{{}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateUDPStats := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_udp_connection_stats",
		ValidMetrics:  []map[string]string{{}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateIfaceStats := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_interface_stats",
		ValidMetrics:  []map[string]string{{}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath,
		},
		Steps: []flow.Steper{validateIPStats, validateUDPStats, validateIfaceStats},
	}

	wf.Add(flow.Step(validateWithPF).DependsOn(dependsOn).Retry(steps.RetryValidation()...))
	return validateWithPF
}
