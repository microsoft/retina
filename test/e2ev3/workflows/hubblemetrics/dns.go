// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	flow "github.com/Azure/go-workflow"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addHubbleDNSScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	agnhostName := "agnhost-dns"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName:        agnhostName,
		AgnhostNamespace:   config.TestPodNamespace,
		AgnhostArch:        arch,
		KubeConfigFilePath: kubeConfigFilePath,
	}
	execNslookup := &k8s.ExecInPod{
		PodName:            agnhostName + "-0",
		PodNamespace:       config.TestPodNamespace,
		Command:            "nslookup -type=a one.one.one.one",
		KubeConfigFilePath: kubeConfigFilePath,
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
			KubeConfigFilePath:    kubeConfigFilePath,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateQuery, validateResponse},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType:       k8s.TypeString(k8s.StatefulSet),
		ResourceName:       agnhostName,
		ResourceNamespace:  config.TestPodNamespace,
		KubeConfigFilePath: kubeConfigFilePath,
	}

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createAgnhost, execNslookup).
			DependsOn(upstream).
			Timeout(utils.DefaultScenarioTimeout),
	)

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(execNslookup).
			Retry(utils.RetryWithBackoff),
	)

	// Cleanup: always runs, even if validation fails.
	wf.Add(
		flow.Pipe(deleteAgnhost).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return deleteAgnhost
}
