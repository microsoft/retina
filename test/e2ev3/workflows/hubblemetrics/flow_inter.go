// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"k8s.io/client-go/rest"
	"log/slog"
)

func addHubbleFlowInterNodeScenario(log *slog.Logger, restConfig *rest.Config, arch string) *flow.Workflow {
	log = log.With("test", "flow-inter")
	wf := &flow.Workflow{DontPanic: true}
	podnameSrc := "agnhost-flow-inter-src"
	podnameDst := "agnhost-flow-inter-dst"
	validSrcLabels := []map[string]string{
		{"source": config.TestPodNamespace + "/" + podnameSrc + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": config.TestPodNamespace + "/" + podnameDst + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}
	// Validate from dst pod's perspective using source-based labels.
	// With sourceEgressContext=pod, flow metrics always populate 'source' with the local pod
	// and leave 'destination' empty — so we check dst-0 appears as source for both directions.
	validDstLabels := []map[string]string{
		{"source": config.TestPodNamespace + "/" + podnameDst + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": config.TestPodNamespace + "/" + podnameDst + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createSrc := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameSrc, AgnhostNamespace: config.TestPodNamespace,
		AgnhostArch: arch, RestConfig: restConfig, Log: log,
	}
	createDst := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameDst, AgnhostNamespace: config.TestPodNamespace,
		AgnhostArch: arch, RestConfig: restConfig, Log: log,
	}
	curlPod := &CurlPodStep{
		SrcPodName: podnameSrc + "-0", SrcPodNamespace: config.TestPodNamespace,
		DstPodName: podnameDst + "-0", DstPodNamespace: config.TestPodNamespace,
		RestConfig: restConfig,
		Log:        log,
	}
	validateSrc := &prom.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validSrcLabels, ExpectMetric: true,
	}
	validateDst := &prom.ValidateMetricStep{
		ForwardedPort: "9966", MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validDstLabels, ExpectMetric: true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
			Namespace: config.KubeSystemNamespace, Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + podnameSrc,
		},
		Steps: []flow.Steper{
			curlPod,
			validateSrc,
			&utils.WithPortForward{
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
		ResourceNamespace: config.TestPodNamespace, RestConfig: restConfig,
	}
	deleteDst := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podnameDst,
		ResourceNamespace: config.TestPodNamespace, RestConfig: restConfig,
	}

	wf.Add(
		flow.BatchPipe(
			// Setup: provision resources.
			flow.Pipe(createSrc, createDst).
				Timeout(utils.DefaultScenarioTimeout),
			// Validate: generate traffic and check metrics, retry with backoff.
			flow.Steps(validateWithPF).
				Retry(utils.RetryWithBackoff),
			// Cleanup: always runs, even if validation fails.
			flow.Pipe(deleteSrc, deleteDst).
				When(flow.Always),
		),
	)
	return wf
}
