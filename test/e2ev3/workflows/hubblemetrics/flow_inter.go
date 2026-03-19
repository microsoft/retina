// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/framework/constants"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addHubbleFlowInterNodeScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	podnameSrc := "agnhost-flow-inter-src"
	podnameDst := "agnhost-flow-inter-dst"
	validSrcLabels := []map[string]string{
		{"source": common.TestPodNamespace + "/" + podnameSrc + "-0", "destination": "", "protocol": constants.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podnameDst + "-0", "destination": "", "protocol": constants.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}
	validDstLabels := []map[string]string{
		{"source": "", "destination": common.TestPodNamespace + "/" + podnameSrc + "-0", "protocol": constants.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": "", "destination": common.TestPodNamespace + "/" + podnameDst + "-0", "protocol": constants.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createSrc := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameSrc, AgnhostNamespace: common.TestPodNamespace,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	createDst := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podnameDst, AgnhostNamespace: common.TestPodNamespace,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep30 := &steps.SleepStep{Duration: 30 * time.Second}
	pfSrc := &k8s.PortForward{
		LabelSelector: "k8s-app=retina", LocalPort: constants.HubbleMetricsPort, RemotePort: constants.HubbleMetricsPort,
		Endpoint: constants.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + podnameSrc,
	}
	pfDst := &k8s.PortForward{
		LabelSelector: "k8s-app=retina", LocalPort: "9966", RemotePort: constants.HubbleMetricsPort,
		Endpoint: constants.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + podnameDst,
	}
	curlPod := &steps.CurlPodStep{
		SrcPodName: podnameSrc + "-0", SrcPodNamespace: common.TestPodNamespace,
		DstPodName: podnameDst + "-0", DstPodNamespace: common.TestPodNamespace,
		KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep5 := &steps.SleepStep{Duration: 5 * time.Second}
	validateSrc := &common.ValidateMetricStep{
		ForwardedPort: constants.HubbleMetricsPort, MetricName: constants.HubbleFlowMetricName,
		ValidMetrics: validSrcLabels, ExpectMetric: true,
	}
	validateDst := &common.ValidateMetricStep{
		ForwardedPort: "9966", MetricName: constants.HubbleFlowMetricName,
		ValidMetrics: validDstLabels, ExpectMetric: true,
	}
	stopPFSrc := &steps.StopPortForwardStep{PF: pfSrc}
	stopPFDst := &steps.StopPortForwardStep{PF: pfDst}
	deleteSrc := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podnameSrc,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteDst := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podnameDst,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(
		createSrc, createDst, sleep30,
		pfSrc, pfDst, curlPod, sleep5,
		validateSrc, validateDst,
		stopPFSrc, stopPFDst,
		deleteSrc, deleteDst,
	).DependsOn(upstream))
	return deleteDst
}
