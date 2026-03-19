// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"context"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/framework/constants"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addHubbleDropScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, arch string) flow.Steper {
	agnhostName := steps.HubbleDropAgnhostName
	podName := steps.HubbleDropPodName

	createNetPol := &k8s.CreateDenyAllNetworkPolicy{
		NetworkPolicyNamespace: common.TestPodNamespace,
		KubeConfigFilePath:     kubeConfigFilePath,
		DenyAllLabelSelector:   "app=" + agnhostName,
	}
	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: common.TestPodNamespace,
		AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep30 := &steps.SleepStep{Duration: 30 * time.Second}
	pfRetina := &k8s.PortForward{
		LabelSelector: "k8s-app=retina", LocalPort: constants.RetinaMetricsPort, RemotePort: constants.RetinaMetricsPort,
		Endpoint: constants.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
	}
	pfHubble := &k8s.PortForward{
		LabelSelector: "k8s-app=retina", LocalPort: constants.HubbleMetricsPort, RemotePort: constants.HubbleMetricsPort,
		Endpoint: constants.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
	}
	sleep5 := &steps.SleepStep{Duration: 5 * time.Second}
	execCurl := flow.Func("hubble-drop-curl-"+arch, func(ctx context.Context) error {
		_ = (&k8s.ExecInPod{
			PodName: podName, PodNamespace: common.TestPodNamespace,
			Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
		}).Do(ctx)
		return nil // error expected
	})
	validateRetinaDrop := &common.ValidateMetricStep{
		ForwardedPort: constants.RetinaMetricsPort, MetricName: constants.RetinaDropMetricName,
		ValidMetrics: []map[string]string{steps.ValidRetinaDropMetricLabels}, ExpectMetric: true,
	}
	validateHubbleDrop := &common.ValidateMetricStep{
		ForwardedPort: constants.HubbleMetricsPort, MetricName: constants.HubbleDropMetricName,
		ValidMetrics: []map[string]string{steps.ValidHubbleDropMetricLabels}, ExpectMetric: true, PartialMatch: true,
	}
	stopPFHubble := &steps.StopPortForwardStep{PF: pfHubble}
	stopPFRetina := &steps.StopPortForwardStep{PF: pfRetina}
	deleteNetPol := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.NetworkPolicy), ResourceName: "deny-all",
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(
		createNetPol, createAgnhost, sleep30,
		pfRetina, pfHubble, sleep5, execCurl,
		validateRetinaDrop, validateHubbleDrop,
		stopPFHubble, stopPFRetina,
		deleteNetPol, deleteAgnhost,
	).DependsOn(upstream))
	return deleteAgnhost
}
