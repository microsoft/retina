// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"context"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addAdvancedDNSScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, namespace, arch, variant string,
	command string, expectError bool,
	reqQuery, reqQueryType, workloadKind string,
	respNumResponse, respQuery, respQueryType, respReturnCode, respResponse string,
) flow.Steper {
	agnhostName := "agnhost-adv-dns-" + variant + "-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCmd1 := flow.Func("adv-dns-"+variant+"-1-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	execCmd2 := flow.Func("adv-dns-"+variant+"-2-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	validateReq := &steps.ValidateAdvancedDNSRequestStep{
		PodNamespace: namespace, PodName: podName, Query: reqQuery, QueryType: reqQueryType,
		WorkloadKind: workloadKind, WorkloadName: agnhostName, KubeConfigFilePath: kubeConfigFilePath,
	}
	validateResp := &steps.ValidateAdvancedDNSResponseStep{
		PodNamespace: namespace, NumResponse: respNumResponse, PodName: podName,
		Query: respQuery, QueryType: respQueryType, Response: respResponse, ReturnCode: respReturnCode,
		WorkloadKind: workloadKind, WorkloadName: agnhostName, KubeConfigFilePath: kubeConfigFilePath,
	}
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

	wf.Add(flow.Pipe(createAgnhost, execCmd1, execCmd2).DependsOn(upstream).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCmd2).Retry(steps.RetryValidation()...))
	wf.Add(flow.Step(deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
