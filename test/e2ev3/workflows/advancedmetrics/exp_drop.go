// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addAdvancedDropScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, namespace, arch string) flow.Steper {
	agnhostName := "agnhost-adv-drop-" + arch
	podName := agnhostName + "-0"

	createNetPol := &k8s.CreateDenyAllNetworkPolicy{
		NetworkPolicyNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath, DenyAllLabelSelector: "app=" + agnhostName,
	}
	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCurl := steps.CurlExpectFail("adv-drop-curl-"+arch, &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace,
		Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	})
	validateDropCount := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_drop_count",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}
	validateDropBytes := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort, MetricName: "networkobservability_adv_drop_bytes",
		ValidMetrics: []map[string]string{{}}, ExpectMetric: true, PartialMatch: true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateDropCount, validateDropBytes},
	}
	deleteNetPol := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.NetworkPolicy), ResourceName: "deny-all",
		ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName,
		ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createNetPol, createAgnhost, execCurl).DependsOn(upstream).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCurl).Retry(steps.RetryValidation()...))
	wf.Add(flow.Pipe(deleteNetPol, deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
