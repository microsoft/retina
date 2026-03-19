// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/framework/constants"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
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
	pf := &k8s.PortForward{
		LabelSelector:         "k8s-app=retina",
		LocalPort:             constants.HubbleMetricsPort,
		RemotePort:            constants.HubbleMetricsPort,
		Namespace:             common.KubeSystemNamespace,
		Endpoint:              "metrics",
		KubeConfigFilePath:    kubeConfigFilePath,
		OptionalLabelAffinity: "app=" + agnhostName,
	}
	execNslookup := &k8s.ExecInPod{
		PodName:            agnhostName + "-0",
		PodNamespace:       common.TestPodNamespace,
		Command:            "nslookup -type=a one.one.one.one",
		KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep5 := &steps.SleepStep{Duration: 5 * time.Second}
	validateQuery := &common.ValidateMetricStep{
		ForwardedPort: constants.HubbleMetricsPort,
		MetricName:    constants.HubbleDNSQueryMetricName,
		ValidMetrics:  []map[string]string{steps.ValidHubbleDNSQueryMetricLabels},
		ExpectMetric:  true,
	}
	validateResponse := &common.ValidateMetricStep{
		ForwardedPort: constants.HubbleMetricsPort,
		MetricName:    constants.HubbleDNSResponseMetricName,
		ValidMetrics:  []map[string]string{steps.ValidHubbleDNSResponseMetricLabels},
		ExpectMetric:  true,
	}
	stopPF := &steps.StopPortForwardStep{PF: pf}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType:       k8s.TypeString(k8s.StatefulSet),
		ResourceName:       agnhostName,
		ResourceNamespace:  common.TestPodNamespace,
		KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(
		createAgnhost, pf, execNslookup, sleep5,
		validateQuery, validateResponse,
		stopPF, deleteAgnhost,
	).DependsOn(upstream))
	return deleteAgnhost
}
