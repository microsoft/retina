// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"context"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
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
	sleep30 := &steps.SleepStep{Duration: 30 * time.Second}
	execCurl1 := flow.Func("drop-curl-1-"+arch, func(ctx context.Context) error {
		_ = (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		return nil
	})
	sleep5a := &steps.SleepStep{Duration: 5 * time.Second}
	execCurl2 := flow.Func("drop-curl-2-"+arch, func(ctx context.Context) error {
		_ = (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		return nil
	})
	pf := &k8s.PortForward{
		Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
		LocalPort: "10093", RemotePort: "10093", Endpoint: "metrics",
		KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
	}
	validateDrop := &steps.ValidateRetinaDropMetricStep{PortForwardedRetinaPort: "10093", Direction: "unknown", Reason: steps.IPTableRuleDrop}
	stopPF := &steps.StopPortForwardStep{PF: pf}
	deleteNetPol := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.NetworkPolicy), ResourceName: "deny-all", ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	chain := []flow.Steper{createNetPol, createAgnhost, sleep30, execCurl1, sleep5a, execCurl2, pf, validateDrop, stopPF, deleteNetPol, deleteAgnhost}
	wf.Add(flow.Pipe(chain...).DependsOn(dependsOn))
	return deleteAgnhost
}
