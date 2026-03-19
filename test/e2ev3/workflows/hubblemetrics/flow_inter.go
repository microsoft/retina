// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addHubbleFlowInterNodeScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	podnameSrc := "agnhost-flow-inter-src"
	podnameDst := "agnhost-flow-inter-dst"
	validSrcLabels := []map[string]string{
		{"source": common.TestPodNamespace + "/" + podnameSrc + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podnameDst + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}
	validDstLabels := []map[string]string{
		{"source": "", "destination": common.TestPodNamespace + "/" + podnameSrc + "-0", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": "", "destination": common.TestPodNamespace + "/" + podnameDst + "-0", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createSrc := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameSrc, AgnhostNamespace: common.TestPodNamespace,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	createDst := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameDst, AgnhostNamespace: common.TestPodNamespace,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	curlPod := &steps.CurlPodStep{
		SrcPodName: podnameSrc + "-0", SrcPodNamespace: common.TestPodNamespace,
		DstPodName: podnameDst + "-0", DstPodNamespace: common.TestPodNamespace,
		KubeConfigFilePath: kubeConfigFilePath,
	}
	validateSrc := &common.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validSrcLabels, ExpectMetric: true,
	}
	validateDst := &common.ValidateMetricStep{
		ForwardedPort: "9966", MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validDstLabels, ExpectMetric: true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + podnameSrc,
		},
		Steps: []flow.Steper{
			validateSrc,
			&steps.WithPortForward{
				PF: &k8s.PortForward{
					LabelSelector: "k8s-app=retina", LocalPort: "9966", RemotePort: config.HubbleMetricsPort,
					Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + podnameDst,
				},
				Steps: []flow.Steper{validateDst},
			},
		},
	}
	deleteSrc := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podnameSrc,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteDst := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podnameDst,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createSrc, createDst, curlPod).DependsOn(upstream).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(curlPod).Retry(steps.RetryValidation()...))
	wf.Add(flow.Pipe(deleteSrc, deleteDst).DependsOn(validateWithPF).When(flow.Always))
	return deleteDst
}
