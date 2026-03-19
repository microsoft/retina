// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addDropScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath, namespace, arch string) flow.Steper {
	agnhostName := "agnhost-drop-" + arch
	podName := agnhostName + "-0"

	createNetPol := &k8s.CreateDenyAllNetworkPolicy{
		NetworkPolicyNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath, DenyAllLabelSelector: "app=" + agnhostName,
	}
	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostNamespace: namespace, AgnhostName: agnhostName, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCurl1 := steps.CurlExpectFail("drop-curl-1-"+arch, &k8s.ExecInPod{
		PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	})
	execCurl2 := steps.CurlExpectFail("drop-curl-2-"+arch, &k8s.ExecInPod{
		PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	})
	validateDrop := &steps.ValidateRetinaDropMetricStep{PortForwardedRetinaPort: "10093", Direction: "unknown", Reason: steps.IPTableRuleDrop}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: "10093", RemotePort: "10093", Endpoint: "metrics",
			KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateDrop},
	}
	deleteNetPol := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.NetworkPolicy), ResourceName: "deny-all", ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	chain := []flow.Steper{createNetPol, createAgnhost, execCurl1, execCurl2}
	wf.Add(flow.Pipe(chain...).DependsOn(dependsOn).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCurl2).Retry(steps.RetryValidation()...))
	wf.Add(flow.Pipe(deleteNetPol, deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
