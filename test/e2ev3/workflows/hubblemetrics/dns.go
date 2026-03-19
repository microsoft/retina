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

func addHubbleDNSScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	agnhostName := "agnhost-dns"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName:        agnhostName,
		AgnhostNamespace:   common.TestPodNamespace,
		AgnhostArch:        arch,
		KubeConfigFilePath: kubeConfigFilePath,
	}
	execNslookup := &k8s.ExecInPod{
		PodName:            agnhostName + "-0",
		PodNamespace:       common.TestPodNamespace,
		Command:            "nslookup -type=a one.one.one.one",
		KubeConfigFilePath: kubeConfigFilePath,
	}
	validateQuery := &common.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort,
		MetricName:    config.HubbleDNSQueryMetricName,
		ValidMetrics:  []map[string]string{steps.ValidHubbleDNSQueryMetricLabels},
		ExpectMetric:  true,
	}
	validateResponse := &common.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort,
		MetricName:    config.HubbleDNSResponseMetricName,
		ValidMetrics:  []map[string]string{steps.ValidHubbleDNSResponseMetricLabels},
		ExpectMetric:  true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector:         "k8s-app=retina",
			LocalPort:             config.HubbleMetricsPort,
			RemotePort:            config.HubbleMetricsPort,
			Namespace:             common.KubeSystemNamespace,
			Endpoint:              "metrics",
			KubeConfigFilePath:    kubeConfigFilePath,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateQuery, validateResponse},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType:       k8s.TypeString(k8s.StatefulSet),
		ResourceName:       agnhostName,
		ResourceNamespace:  common.TestPodNamespace,
		KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createAgnhost, execNslookup).DependsOn(upstream).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execNslookup).Retry(steps.RetryValidation()...))
	wf.Add(flow.Step(deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
