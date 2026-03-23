// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"k8s.io/client-go/rest"
	flow "github.com/Azure/go-workflow"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addHubbleFlowInterNodeScenario(restConfig *rest.Config, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	podnameSrc := "agnhost-flow-inter-src"
	podnameDst := "agnhost-flow-inter-dst"
	validSrcLabels := []map[string]string{
		{"source": config.TestPodNamespace + "/" + podnameSrc + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": config.TestPodNamespace + "/" + podnameDst + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}
	validDstLabels := []map[string]string{
		{"source": "", "destination": config.TestPodNamespace + "/" + podnameSrc + "-0", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": "", "destination": config.TestPodNamespace + "/" + podnameDst + "-0", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createSrc := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameSrc, AgnhostNamespace: config.TestPodNamespace,
		AgnhostArch: arch, RestConfig: restConfig,
	}
	createDst := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameDst, AgnhostNamespace: config.TestPodNamespace,
		AgnhostArch: arch, RestConfig: restConfig,
	}
	curlPod := &CurlPodStep{
		SrcPodName: podnameSrc + "-0", SrcPodNamespace: config.TestPodNamespace,
		DstPodName: podnameDst + "-0", DstPodNamespace: config.TestPodNamespace,
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
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
			Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + podnameSrc,
		},
		Steps: []flow.Steper{
			validateSrc,
			&utils.WithPortForward{
				PF: &k8s.PortForward{
					LabelSelector: "k8s-app=retina", LocalPort: "9966", RemotePort: config.HubbleMetricsPort,
					Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + podnameDst,
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

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createSrc, createDst, curlPod).
			Timeout(utils.DefaultScenarioTimeout),
	)

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(curlPod).
			Retry(utils.RetryWithBackoff),
	)

	// Cleanup: always runs, even if validation fails.
	wf.Add(
		flow.Pipe(deleteSrc, deleteDst).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return wf
}
