// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addTCPStatsScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath, namespace, arch string) flow.Steper {
	agnhostName := "agnhost-tcpstats-" + arch
	podName := agnhostName + "-0"

	createKapinger := &k8s.CreateKapingerDeployment{
		KapingerNamespace: namespace, KapingerReplicas: "1", KubeConfigFilePath: kubeConfigFilePath,
	}
	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	waitKapinger := &k8s.WaitPodsReady{
		KubeConfigFilePath: kubeConfigFilePath,
		Namespace:          namespace,
		LabelSelector:      "app=kapinger",
	}
	execCurl1 := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace, Command: "curl -s -m 5 kapinger:80", KubeConfigFilePath: kubeConfigFilePath,
	}
	execCurl2 := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace, Command: "curl -s -m 5 kapinger:80", KubeConfigFilePath: kubeConfigFilePath,
	}
	validateConnStats := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_tcp_connection_stats",
		ValidMetrics:  []map[string]string{{}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateFlagGauges := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_tcp_flag_gauges",
		ValidMetrics:  []map[string]string{{"flag": config.SYN}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateConnStats, validateFlagGauges},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteKapinger := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.Deployment), ResourceName: "kapinger", ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createKapinger, createAgnhost, waitKapinger, execCurl1, execCurl2).DependsOn(dependsOn).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCurl2).Retry(steps.RetryValidation()...))
	wf.Add(flow.Pipe(deleteAgnhost, deleteKapinger).DependsOn(validateWithPF).When(flow.Always))
	return deleteKapinger
}
