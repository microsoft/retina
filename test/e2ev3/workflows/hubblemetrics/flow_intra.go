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

func addHubbleFlowIntraNodeScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	podname := "agnhost-flow-intra"
	replicas := 2
	validLabels := []map[string]string{
		{"source": common.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": constants.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": constants.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podname + "-1", "destination": "", "protocol": constants.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podname + "-1", "destination": "", "protocol": constants.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podname, AgnhostNamespace: common.TestPodNamespace,
		ScheduleOnSameNode: true, AgnhostReplicas: &replicas,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep30 := &steps.SleepStep{Duration: 30 * time.Second}
	pf := &k8s.PortForward{
		LabelSelector: "k8s-app=retina", LocalPort: constants.HubbleMetricsPort, RemotePort: constants.HubbleMetricsPort,
		Endpoint: constants.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + podname,
	}
	curlPod := &steps.CurlPodStep{
		SrcPodName: podname + "-0", SrcPodNamespace: common.TestPodNamespace,
		DstPodName: podname + "-1", DstPodNamespace: common.TestPodNamespace,
		KubeConfigFilePath: kubeConfigFilePath,
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
		createAgnhost, sleep30, pf, curlPod, sleep5,
		validateFlow, stopPF, deleteAgnhost,
	).DependsOn(upstream))
	return deleteAgnhost
}
