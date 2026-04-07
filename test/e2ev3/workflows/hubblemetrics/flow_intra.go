// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"k8s.io/client-go/rest"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
)

func addHubbleFlowIntraNodeScenario(restConfig *rest.Config, namespace string, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	podname := "agnhost-flow-intra"
	replicas := 2
	validLabels := []map[string]string{
		{"source": namespace + "/" + podname + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": namespace + "/" + podname + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
		{"source": namespace + "/" + podname + "-1", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": namespace + "/" + podname + "-1", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podname, AgnhostNamespace: namespace,
		ScheduleOnSameNode: true, AgnhostReplicas: &replicas,
		AgnhostArch: arch, RestConfig: restConfig,
	}
	curlPod := &CurlPodStep{
		SrcPodName: podname + "-0", SrcPodNamespace: namespace,
		DstPodName: podname + "-1", DstPodNamespace: namespace,
		RestConfig: restConfig,
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
		Steps: []flow.Steper{curlPod, validateFlow},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podname,
		ResourceNamespace: namespace, RestConfig: restConfig,
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
