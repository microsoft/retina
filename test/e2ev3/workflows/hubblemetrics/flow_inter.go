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

func addHubbleFlowInterNodeScenario(restConfig *rest.Config, namespace string, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	podnameSrc := "agnhost-flow-inter-src"
	podnameDst := "agnhost-flow-inter-dst"
	validSrcLabels := []map[string]string{
		{"source": namespace + "/" + podnameSrc + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": namespace + "/" + podnameDst + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}
	// Validate from dst pod's perspective using source-based labels.
	// With sourceEgressContext=pod, flow metrics always populate 'source' with the local pod
	// and leave 'destination' empty — so we check dst-0 appears as source for both directions.
	validDstLabels := []map[string]string{
		{"source": namespace + "/" + podnameDst + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": namespace + "/" + podnameDst + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createSrc := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameSrc, AgnhostNamespace: namespace,
		AgnhostArch: arch, RestConfig: restConfig,
	}
	createDst := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameDst, AgnhostNamespace: namespace,
		AgnhostArch: arch, RestConfig: restConfig,
	}
	curlPod := &CurlPodStep{
		SrcPodName: podnameSrc + "-0", SrcPodNamespace: namespace,
		DstPodName: podnameDst + "-0", DstPodNamespace: namespace,
		RestConfig: restConfig,
	}
	validateSrc := &prom.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validSrcLabels, ExpectMetric: true,
	}
	validateDst := &prom.ValidateMetricStep{
		ForwardedPort: "9966", MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validDstLabels, ExpectMetric: true,
	}
	validateWithPF := &k8s.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
			Namespace: config.KubeSystemNamespace, Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + podnameSrc,
		},
		Steps: []flow.Steper{
			curlPod,
			validateSrc,
			&k8s.WithPortForward{
				PF: &k8s.PortForward{
					LabelSelector: "k8s-app=retina", LocalPort: "9966", RemotePort: config.HubbleMetricsPort,
					Namespace: config.KubeSystemNamespace, Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + podnameDst,
				},
				Steps: []flow.Steper{validateDst},
			},
		},
	}
	deleteSrc := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podnameSrc,
		ResourceNamespace: namespace, RestConfig: restConfig,
	}
	deleteDst := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podnameDst,
		ResourceNamespace: namespace, RestConfig: restConfig,
	}

	wf.Add(
		flow.BatchPipe(
			// Setup: provision resources.
			flow.Pipe(createSrc, createDst).
				Timeout(k8s.DefaultScenarioTimeout),
			// Validate: generate traffic and check metrics, retry with backoff.
			flow.Steps(validateWithPF).
				Retry(k8s.RetryWithBackoff),
			// Cleanup: always runs, even if validation fails.
			flow.Pipe(deleteSrc, deleteDst).
				When(flow.Always),
		),
	)
	return wf
}
