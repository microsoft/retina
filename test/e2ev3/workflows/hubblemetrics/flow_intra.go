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

func addHubbleFlowIntraNodeScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	podname := "agnhost-flow-intra"
	replicas := 2
	validLabels := []map[string]string{
		{"source": common.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podname + "-1", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": common.TestPodNamespace + "/" + podname + "-1", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podname, AgnhostNamespace: common.TestPodNamespace,
		ScheduleOnSameNode: true, AgnhostReplicas: &replicas,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	curlPod := &steps.CurlPodStep{
		SrcPodName: podname + "-0", SrcPodNamespace: common.TestPodNamespace,
		DstPodName: podname + "-1", DstPodNamespace: common.TestPodNamespace,
		KubeConfigFilePath: kubeConfigFilePath,
	}
	validateFlow := &common.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validLabels, ExpectMetric: true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + podname,
		},
		Steps: []flow.Steper{validateFlow},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podname,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createAgnhost, curlPod).DependsOn(upstream).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(curlPod).Retry(steps.RetryValidation()...))
	wf.Add(flow.Step(deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
