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
	execCurl := steps.CurlExpectFail("hubble-drop-curl-"+arch, &k8s.ExecInPod{
		PodName: podName, PodNamespace: common.TestPodNamespace,
		Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	})
	validateRetinaDrop := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: config.RetinaDropMetricName,
		ValidMetrics: []map[string]string{steps.ValidRetinaDropMetricLabels}, ExpectMetric: true,
	}
	validateHubbleDrop := &common.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleDropMetricName,
		ValidMetrics: []map[string]string{steps.ValidHubbleDropMetricLabels}, ExpectMetric: true, PartialMatch: true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{
			validateRetinaDrop,
			&steps.WithPortForward{
				PF: &k8s.PortForward{
					LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
					Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
				},
				Steps: []flow.Steper{validateHubbleDrop},
			},
		},
	}
	deleteNetPol := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.NetworkPolicy), ResourceName: "deny-all",
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName,
		ResourceNamespace: common.TestPodNamespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createNetPol, createAgnhost, execCurl).DependsOn(upstream).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCurl).Retry(steps.RetryValidation()...))
	wf.Add(flow.Pipe(deleteNetPol, deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
