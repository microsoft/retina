// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addAPIServerLatencyScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath string) flow.Steper {
	validateLatency := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_node_apiserver_latency",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}
	validateNoResponse := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_node_apiserver_no_response",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "k8s-app=retina",
		},
		Steps: []flow.Steper{validateLatency, validateNoResponse},
	}

	wf.Add(flow.Step(validateWithPF).DependsOn(upstream).Retry(steps.RetryValidation()...))
	return validateWithPF
}
