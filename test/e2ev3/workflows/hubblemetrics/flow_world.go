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

func addHubbleFlowToWorldScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	podname := "agnhost-flow-world"
	validLabels := []map[string]string{
		{"source": common.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": constants.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": constants.UDP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
	}

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podname, AgnhostNamespace: common.TestPodNamespace,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep30 := &steps.SleepStep{Duration: 30 * time.Second}
	pf := &k8s.PortForward{
		LabelSelector: "k8s-app=retina", LocalPort: constants.HubbleMetricsPort, RemotePort: constants.HubbleMetricsPort,
		Endpoint: constants.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + podname,
	}
	execCurl := &k8s.ExecInPod{
		PodName: podname + "-0", PodNamespace: common.TestPodNamespace,
		Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep5 := &steps.SleepStep{Duration: 5 * time.Second}
	validateFlow := &common.ValidateMetricStep{
		ForwardedPort: constants.HubbleMetricsPort, MetricName: constants.HubbleFlowMetricName,
		ValidMetrics: validLabels, ExpectMetric: true,
	}
	stopPF := &steps.StopPortForwardStep{PF: pf}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podname,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(
		createAgnhost, sleep30, pf, execCurl, sleep5,
		validateFlow, stopPF, deleteAgnhost,
	).DependsOn(upstream))
	return deleteAgnhost
}
