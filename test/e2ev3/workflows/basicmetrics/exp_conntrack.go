// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"context"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addConntrackScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath, namespace, arch string) flow.Steper {
	agnhostName := "agnhost-ct-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCurl1 := flow.Func("ct-curl-1-"+arch, func(ctx context.Context) error {
		return (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
	})
	execCurl2 := flow.Func("ct-curl-2-"+arch, func(ctx context.Context) error {
		return (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
	})
	conntrackMetrics := []string{
		"networkobservability_conntrack_packets_tx",
		"networkobservability_conntrack_packets_rx",
		"networkobservability_conntrack_bytes_tx",
		"networkobservability_conntrack_bytes_rx",
		"networkobservability_conntrack_total_connections",
	}

	validateSteps := make([]flow.Steper, 0, len(conntrackMetrics))
	for _, metric := range conntrackMetrics {
		validateSteps = append(validateSteps, &common.ValidateMetricStep{
			ForwardedPort: config.RetinaMetricsPort,
			MetricName:    metric,
			ValidMetrics:  []map[string]string{{}},
			ExpectMetric:  true,
			PartialMatch:  true,
		})
	}

	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: validateSteps,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createAgnhost, execCurl1, execCurl2).DependsOn(dependsOn).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCurl2).Retry(steps.RetryValidation()...))
	wf.Add(flow.Step(deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
