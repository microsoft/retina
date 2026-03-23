// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	"context"
	"k8s.io/client-go/rest"
	"log/slog"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addForwardScenario(log *slog.Logger, restConfig *rest.Config, namespace, arch string) *flow.Workflow {
	log = log.With("test", "forward")
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := "agnhost-fwd-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, RestConfig: restConfig, Log: log,
	}
	execCurl1 := flow.Func("fwd-curl-1-"+arch, func(ctx context.Context) error {
		return (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", RestConfig: restConfig}).Do(ctx)
	})
	execCurl2 := flow.Func("fwd-curl-2-"+arch, func(ctx context.Context) error {
		return (&k8s.ExecInPod{PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", RestConfig: restConfig}).Do(ctx)
	})
	validateFwdCount := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_forward_count",
		ValidMetrics:  []map[string]string{{"direction": "egress"}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateFwdBytes := &prom.ValidateMetricStep{
		ForwardedPort: config.RetinaMetricsPort,
		MetricName:    "networkobservability_forward_bytes",
		ValidMetrics:  []map[string]string{{"direction": "egress"}},
		ExpectMetric:  true,
		PartialMatch:  true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: config.MetricsEndpoint, RestConfig: restConfig,
			OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateFwdCount, validateFwdBytes},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, RestConfig: restConfig,
	}

	wf.Add(
		flow.BatchPipe(
			// Setup: provision resources and generate traffic.
			flow.Pipe(createAgnhost, execCurl1, execCurl2).
				Timeout(utils.DefaultScenarioTimeout),
			// Validate: retry with exponential backoff until metrics appear.
			flow.Steps(validateWithPF).
				Retry(utils.RetryWithBackoff),
			// Cleanup: always runs, even if validation fails.
			flow.Pipe(deleteAgnhost).
				When(flow.Always),
		),
	)
	return wf
}
