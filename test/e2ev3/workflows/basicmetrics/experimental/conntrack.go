// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	"k8s.io/client-go/rest"
	"context"

	flow "github.com/Azure/go-workflow"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addConntrackScenario(restConfig *rest.Config, namespace, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := "agnhost-ct-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, RestConfig: restConfig,
	}
	execCurl1 := flow.Func("ct-curl-1-"+arch, func(ctx context.Context) error {
		return (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", RestConfig: restConfig}).Do(ctx)
	})
	execCurl2 := flow.Func("ct-curl-2-"+arch, func(ctx context.Context) error {
		return (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", RestConfig: restConfig}).Do(ctx)
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
		validateSteps = append(validateSteps, &prom.ValidateMetricStep{
			ForwardedPort: config.RetinaMetricsPort,
			MetricName:    metric,
			ValidMetrics:  []map[string]string{{}},
			ExpectMetric:  true,
			PartialMatch:  true,
		})
	}

	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, RestConfig: restConfig,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: validateSteps,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, RestConfig: restConfig,
	}

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createAgnhost, execCurl1, execCurl2).
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
		flow.Pipe(deleteAgnhost).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return wf
}
