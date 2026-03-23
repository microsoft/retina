// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"k8s.io/client-go/rest"
	"log/slog"
)

func addHubbleDNSScenario(log *slog.Logger, restConfig *rest.Config, arch string) *flow.Workflow {
	log = log.With("test", "dns")
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := "agnhost-dns"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName:      agnhostName,
		AgnhostNamespace: config.TestPodNamespace,
		AgnhostArch:      arch,
		RestConfig:       restConfig,
		Log:              log,
	}
	execNslookup := &k8s.ExecInPod{
		PodName:      agnhostName + "-0",
		PodNamespace: config.TestPodNamespace,
		Command:      "nslookup -type=a one.one.one.one",
		RestConfig:   restConfig,
	}
	validateQuery := &prom.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort,
		MetricName:    config.HubbleDNSQueryMetricName,
		ValidMetrics:  []map[string]string{ValidHubbleDNSQueryMetricLabels},
		ExpectMetric:  true,
	}
	validateResponse := &prom.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort,
		MetricName:    config.HubbleDNSResponseMetricName,
		ValidMetrics:  []map[string]string{ValidHubbleDNSResponseMetricLabels},
		ExpectMetric:  true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector:         "k8s-app=retina",
			LocalPort:             config.HubbleMetricsPort,
			RemotePort:            config.HubbleMetricsPort,
			Namespace:             config.KubeSystemNamespace,
			Endpoint:              "metrics",
			RestConfig:            restConfig,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{execNslookup, validateQuery, validateResponse},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType:      k8s.TypeString(k8s.StatefulSet),
		ResourceName:      agnhostName,
		ResourceNamespace: config.TestPodNamespace,
		RestConfig:        restConfig,
	}

	wf.Add(
		flow.BatchPipe(
			// Setup: provision resources.
			flow.Pipe(createAgnhost).
				Timeout(utils.DefaultScenarioTimeout),
			// Validate: generate traffic and check metrics, retry with backoff.
			flow.Steps(validateWithPF).
				Retry(utils.RetryWithBackoff),
			// Cleanup: always runs, even if validation fails.
			flow.Pipe(deleteAgnhost).
				When(flow.Always),
		),
	)
	return wf
}
