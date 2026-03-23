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
)

func addHubbleDropScenario(restConfig *rest.Config, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := HubbleDropAgnhostName
	podName := HubbleDropPodName

	createNetPol := &k8s.CreateDenyAllNetworkPolicy{
		NetworkPolicyNamespace: config.TestPodNamespace,
		RestConfig:             restConfig,
		DenyAllLabelSelector:   "app=" + agnhostName,
	}
	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: config.TestPodNamespace,
		AgnhostArch: arch, RestConfig: restConfig,
	}
	execCurl := utils.CurlExpectFail("hubble-drop-curl-"+arch, &k8s.ExecInPod{
		PodName: podName, PodNamespace: config.TestPodNamespace,
		Command: "curl -s -m 5 bing.com", RestConfig: restConfig,
	})
	validateRetinaDrop := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: config.RetinaDropMetricName,
		ValidMetrics: []map[string]string{ValidRetinaDropMetricLabels}, ExpectMetric: true,
	}
	validateHubbleDrop := &prom.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleDropMetricName,
		ValidMetrics: []map[string]string{ValidHubbleDropMetricLabels}, ExpectMetric: true, PartialMatch: true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Namespace: config.KubeSystemNamespace, Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{
			execCurl,
			validateRetinaDrop,
			&utils.WithPortForward{
				PF: &k8s.PortForward{
					LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
					Namespace: config.KubeSystemNamespace, Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + agnhostName,
				},
				Steps: []flow.Steper{validateHubbleDrop},
			},
		},
	}
	deleteNetPol := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.NetworkPolicy), ResourceName: "deny-all",
		ResourceNamespace: config.TestPodNamespace, RestConfig: restConfig,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName,
		ResourceNamespace: config.TestPodNamespace, RestConfig: restConfig,
	}

	wf.Add(
		flow.BatchPipe(
			// Setup: provision resources.
			flow.Pipe(createNetPol, createAgnhost).
				Timeout(utils.DefaultScenarioTimeout),
			// Validate: generate traffic and check metrics, retry with backoff.
			flow.Steps(validateWithPF).
				Retry(utils.RetryWithBackoff),
			// Cleanup: always runs, even if validation fails.
			flow.Pipe(deleteNetPol, deleteAgnhost).
				When(flow.Always),
		),
	)
	return wf
}
