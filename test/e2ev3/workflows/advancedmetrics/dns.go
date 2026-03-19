// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"context"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/framework/constants"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
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
	sleep30 := &steps.SleepStep{Duration: 30 * time.Second}
	execCmd1 := flow.Func("adv-dns-"+variant+"-1-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	sleep5a := &steps.SleepStep{Duration: 5 * time.Second}
	execCmd2 := flow.Func("adv-dns-"+variant+"-2-"+arch, func(ctx context.Context) error {
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
	validateReq := &steps.ValidateAdvancedDNSRequestStep{
		PodNamespace: namespace, PodName: podName, Query: reqQuery, QueryType: reqQueryType,
		WorkloadKind: workloadKind, WorkloadName: agnhostName, KubeConfigFilePath: kubeConfigFilePath,
	}
	validateResp := &steps.ValidateAdvancedDNSResponseStep{
		PodNamespace: namespace, NumResponse: respNumResponse, PodName: podName,
		Query: respQuery, QueryType: respQueryType, Response: respResponse, ReturnCode: respReturnCode,
		WorkloadKind: workloadKind, WorkloadName: agnhostName, KubeConfigFilePath: kubeConfigFilePath,
	}
	stopPF := &steps.StopPortForwardStep{PF: pf}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	sleep5c := &steps.SleepStep{Duration: 5 * time.Second}

	chain := []flow.Steper{createAgnhost, sleep30, execCmd1, sleep5a, execCmd2, sleep5b, pf, validateReq, validateResp, stopPF, deleteAgnhost, sleep5c}
	wf.Add(flow.Pipe(chain...).DependsOn(upstream))
	return sleep5c
}
