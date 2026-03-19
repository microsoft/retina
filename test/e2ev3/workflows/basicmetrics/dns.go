// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"context"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addBasicDNSScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath, namespace, arch, variant, command string, expectError bool) flow.Steper {
	agnhostName := "agnhost-dns-basic-" + variant + "-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCmd1 := flow.Func("basic-dns-"+variant+"-1-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	execCmd2 := flow.Func("basic-dns-"+variant+"-2-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	validateReq := &steps.ValidateBasicDNSRequestStep{}
	validateResp := &steps.ValidateBasicDNSResponseStep{}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: "metrics", KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateReq, validateResp},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	chain := []flow.Steper{createAgnhost, execCmd1, execCmd2}
	wf.Add(flow.Pipe(chain...).DependsOn(dependsOn).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCmd2).Retry(steps.RetryValidation()...))
	wf.Add(flow.Steps(deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
