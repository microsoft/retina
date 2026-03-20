// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	"k8s.io/client-go/rest"
	flow "github.com/Azure/go-workflow"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addTCPStatsScenario(restConfig *rest.Config, namespace, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := "agnhost-tcpstats-" + arch
	podName := agnhostName + "-0"

	createKapinger := &k8s.CreateKapingerDeployment{
		KapingerNamespace: namespace, KapingerReplicas: "1", RestConfig: restConfig,
	}
	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, RestConfig: restConfig,
	}
	waitKapinger := &k8s.WaitPodsReady{
		RestConfig: restConfig,
		Namespace:          namespace,
		LabelSelector:      "app=kapinger",
	}
	execCurl1 := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace, Command: "curl -s -m 5 kapinger:80", RestConfig: restConfig,
	}
	execCurl2 := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace, Command: "curl -s -m 5 kapinger:80", RestConfig: restConfig,
	}
	validateConnStats := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_tcp_connection_stats",
		ValidMetrics:  []map[string]string{{}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateFlagGauges := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_tcp_flag_gauges",
		ValidMetrics:  []map[string]string{{"flag": config.SYN}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, RestConfig: restConfig,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateConnStats, validateFlagGauges},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, RestConfig: restConfig,
	}
	deleteKapinger := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.Deployment), ResourceName: "kapinger", ResourceNamespace: namespace, RestConfig: restConfig,
	}

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createKapinger, createAgnhost, waitKapinger, execCurl1, execCurl2).
			Timeout(utils.DefaultScenarioTimeout),
	)

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(execCurl2).
			Retry(utils.RetryWithBackoff),
	)

	// Cleanup: always runs, even if validation fails.
	wf.Add(
		flow.Pipe(deleteAgnhost, deleteKapinger).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return wf
}
