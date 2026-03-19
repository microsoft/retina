// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addAdvancedTCPScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, namespace, arch string) flow.Steper {
	agnhostName := "agnhost-adv-tcp-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCurl := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace,
		Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	}
	validateTCPFlags := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_tcpflags_count",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}
	validateTCPRetrans := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_tcpretrans_count",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateTCPFlags, validateTCPRetrans},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName,
		ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createAgnhost, execCurl).DependsOn(upstream).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCurl).Retry(steps.RetryValidation()...))
	wf.Add(flow.Step(deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
