// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"context"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/framework/constants"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addBasicDNSScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath, namespace, arch, variant, command string, expectError bool) flow.Steper {
	agnhostName := "agnhost-dns-basic-" + variant + "-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep30 := &steps.SleepStep{Duration: 30 * time.Second}
	execCmd1 := flow.Func("basic-dns-"+variant+"-1-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	sleep5a := &steps.SleepStep{Duration: 5 * time.Second}
	execCmd2 := flow.Func("basic-dns-"+variant+"-2-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	sleep5b := &steps.SleepStep{Duration: 5 * time.Second}
	pf := &k8s.PortForward{
		Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
		LocalPort: constants.RetinaMetricsPort, RemotePort: constants.RetinaMetricsPort,
		Endpoint: "metrics", KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
	}
	validateReq := &steps.ValidateBasicDNSRequestStep{}
	validateResp := &steps.ValidateBasicDNSResponseStep{}
	stopPF := &steps.StopPortForwardStep{PF: pf}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep5c := &steps.SleepStep{Duration: 5 * time.Second}

	chain := []flow.Steper{createAgnhost, sleep30, execCmd1, sleep5a, execCmd2, sleep5b, pf, validateReq, validateResp, stopPF, deleteAgnhost, sleep5c}
	wf.Add(flow.Pipe(chain...).DependsOn(dependsOn))
	return sleep5c
}
