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

func addHubbleTCPScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	agnhostName := "agnhost-tcp"
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: common.TestPodNamespace,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep30 := &steps.SleepStep{Duration: 30 * time.Second}
	pf := &k8s.PortForward{
		LabelSelector: "k8s-app=retina", LocalPort: constants.HubbleMetricsPort, RemotePort: constants.HubbleMetricsPort,
		Namespace: common.KubeSystemNamespace, Endpoint: constants.MetricsEndpoint,
		KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
	}
	execCurl := &k8s.ExecInPod{
		PodName: podName, PodNamespace: common.TestPodNamespace,
		Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep5 := &steps.SleepStep{Duration: 5 * time.Second}
	validateTCP := &common.ValidateMetricStep{
		ForwardedPort: constants.HubbleMetricsPort, MetricName: constants.HubbleTCPFlagsMetricName,
		ValidMetrics: steps.ValidHubbleTCPMetricsLabels, ExpectMetric: true,
	}
	stopPF := &steps.StopPortForwardStep{PF: pf}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(
		createAgnhost, sleep30, pf, execCurl, sleep5,
		validateTCP, stopPF, deleteAgnhost,
	).DependsOn(upstream))
	return deleteAgnhost
}
