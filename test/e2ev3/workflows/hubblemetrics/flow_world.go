// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	flow "github.com/Azure/go-workflow"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addHubbleFlowToWorldScenario(kubeConfigFilePath, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	podname := "agnhost-flow-world"
	validLabels := []map[string]string{
		{"source": config.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": config.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": config.UDP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
	}

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podname, AgnhostNamespace: config.TestPodNamespace,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCurl := &k8s.ExecInPod{
		PodName: podname + "-0", PodNamespace: config.TestPodNamespace,
		Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	}
	validateFlow := &prom.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validLabels, ExpectMetric: true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + podname,
		},
		Steps: []flow.Steper{validateFlow},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podname,
		ResourceNamespace: config.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
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
