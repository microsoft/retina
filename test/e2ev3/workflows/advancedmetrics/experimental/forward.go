// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	"k8s.io/client-go/rest"
	flow "github.com/Azure/go-workflow"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addAdvancedForwardScenario(restConfig *rest.Config, namespace, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := "agnhost-adv-fwd-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, RestConfig: restConfig,
	}
	execCurl := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace,
		Command: "curl -s -m 5 bing.com", RestConfig: restConfig,
	}
	validateForwardCount := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_forward_count",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}
	validateForwardBytes := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_forward_bytes",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateForwardCount, validateForwardBytes},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName,
		ResourceNamespace: namespace, RestConfig: restConfig,
	}

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createAgnhost, execCurl).
			Timeout(utils.DefaultScenarioTimeout),
	)

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(execCurl).
			Retry(utils.RetryWithBackoff),
	)

	// Cleanup: always runs, even if validation fails.
	wf.Add(
		flow.Pipe(deleteAgnhost).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return wf
}
