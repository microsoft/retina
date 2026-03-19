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

func addForwardScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath, namespace, arch string) flow.Steper {
	agnhostName := "agnhost-fwd-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCurl1 := flow.Func("fwd-curl-1-"+arch, func(ctx context.Context) error {
		return (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
	})
	execCurl2 := flow.Func("fwd-curl-2-"+arch, func(ctx context.Context) error {
		return (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
	})
	validateFwdCount := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_forward_count",
		ValidMetrics:  []map[string]string{{"direction": "egress"}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateFwdBytes := &common.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_forward_bytes",
		ValidMetrics:  []map[string]string{{"direction": "egress"}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, KubeConfigFilePath: kubeConfigFilePath,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateFwdCount, validateFwdBytes},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(createAgnhost, execCurl1, execCurl2).DependsOn(dependsOn).Timeout(10 * time.Minute))
	wf.Add(flow.Step(validateWithPF).DependsOn(execCurl2).Retry(steps.RetryValidation()...))
	wf.Add(flow.Step(deleteAgnhost).DependsOn(validateWithPF).When(flow.Always))
	return deleteAgnhost
}
