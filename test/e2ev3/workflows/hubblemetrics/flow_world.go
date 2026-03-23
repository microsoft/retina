// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"k8s.io/client-go/rest"
)

func addHubbleFlowToWorldScenario(restConfig *rest.Config, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	podname := "agnhost-flow-world"
	validLabels := []map[string]string{
		{"source": config.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": config.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": config.UDP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
	}

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podname, AgnhostNamespace: config.TestPodNamespace,
		AgnhostArch: arch, RestConfig: restConfig,
	}
	execCurl := &k8s.ExecInPod{
		PodName: podname + "-0", PodNamespace: config.TestPodNamespace,
		Command: "curl -s -m 5 bing.com", RestConfig: restConfig,
	}
	validateFlow := &prom.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validLabels, ExpectMetric: true,
	}
	validateWithPF := &k8s.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
			Namespace: config.KubeSystemNamespace, Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + podname,
		},
		Steps: []flow.Steper{execCurl, validateFlow},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podname,
		ResourceNamespace: config.TestPodNamespace, RestConfig: restConfig,
	}

	wf.Add(
		flow.BatchPipe(
			// Setup: provision resources.
			flow.Pipe(createAgnhost).
				Timeout(k8s.DefaultScenarioTimeout),
			// Validate: generate traffic and check metrics, retry with backoff.
			flow.Steps(validateWithPF).
				Retry(k8s.RetryWithBackoff),
			// Cleanup: always runs, even if validation fails.
			flow.Pipe(deleteAgnhost).
				When(flow.Always),
		),
	)
	return wf
}
