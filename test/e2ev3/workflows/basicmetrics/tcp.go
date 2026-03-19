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

func addTCPScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath, namespace, arch string) flow.Steper {
	agnhostName := "agnhost-tcp-" + arch
	podName := agnhostName + "-0"

	createKapinger := &k8s.CreateKapingerDeployment{
		KapingerNamespace: namespace, KapingerReplicas: "1", KubeConfigFilePath: kubeConfigFilePath,
	}
	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	waitKapinger := &k8s.WaitPodsReady{
		KubeConfigFilePath: kubeConfigFilePath,
		Namespace:          namespace,
		LabelSelector:      "app=kapinger",
	}
	execCurl1 := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	}
	execCurl2 := &k8s.ExecInPod{
		PodName: podName, PodNamespace: namespace, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	}
	validateState := &steps.ValidateRetinaTCPStateStep{PortForwardedRetinaPort: "10093"}
	validateRemote := &steps.ValidateRetinaTCPConnectionRemoteStep{PortForwardedRetinaPort: "10093"}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: "10093", RemotePort: "10093", Endpoint: "metrics",
			KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateState, validateRemote},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	chain := []flow.Steper{createKapinger, createAgnhost, waitKapinger, execCurl1, execCurl2}
	wf.Add(flow.Pipe(chain...).DependsOn(dependsOn).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCurl2).Retry(steps.RetryValidation()...))
	wf.Add(flow.Steps(deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
