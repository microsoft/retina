// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	"log/slog"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"k8s.io/client-go/rest"
)

func addAdvancedTCPScenario(restConfig *rest.Config, namespace, arch string, skipRetrans bool, cfg *config.E2EConfig) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := "agnhost-adv-tcp-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, RestConfig: restConfig,
	}
	execCurl := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace,
		Command: "curl -s -m 5 bing.com", RestConfig: restConfig,
	}
	validateTCPFlags := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_tcpflags_count",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}

	validateSteps := []flow.Steper{validateTCPFlags}
	if skipRetrans {
		reason := "retransmissions are near-zero on Kind's local network"
		slog.Info("SKIP: adv_tcpretrans_count — " + reason)
		if cfg.Summary != nil {
			cfg.Summary.Skip("advanced-metrics-experimental", "adv_tcpretrans_count", reason)
		}
	} else {
		validateTCPRetrans := &prom.ValidateMetricStep{
			ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_tcpretrans_count",
			ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
		}
		validateSteps = append(validateSteps, validateTCPRetrans)
	}

	validateWithPF := &k8s.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: validateSteps,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName,
		ResourceNamespace: namespace, RestConfig: restConfig,
	}

	wf.Add(
		flow.BatchPipe(
			// Setup: provision resources and generate traffic.
			flow.Pipe(createAgnhost, execCurl).Timeout(k8s.DefaultScenarioTimeout),
			// Validate: retry with exponential backoff until metrics appear.
			flow.Steps(validateWithPF).Retry(k8s.RetryWithBackoff),
			// Cleanup: always runs, even if validation fails.
			flow.Pipe(deleteAgnhost).When(flow.Always),
		),
	)
	return wf
}
