// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"context"
	"fmt"
	"log/slog"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"k8s.io/client-go/rest"
)

func addDropScenario(log *slog.Logger, restConfig *rest.Config, namespace, arch string) *flow.Workflow {
	log = log.With("test", "drop")
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := "agnhost-drop-" + arch
	podName := agnhostName + "-0"

	createNetPol := &k8s.CreateDenyAllNetworkPolicy{
		NetworkPolicyNamespace: namespace, RestConfig: restConfig, DenyAllLabelSelector: "app=" + agnhostName, Log: log,
	}
	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostNamespace: namespace, AgnhostName: agnhostName, AgnhostArch: arch, RestConfig: restConfig, Log: log,
	}
	execCurl1 := utils.CurlExpectFail("drop-curl-1-"+arch, &k8s.ExecInPod{
		PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", RestConfig: restConfig,
	})
	execCurl2 := utils.CurlExpectFail("drop-curl-2-"+arch, &k8s.ExecInPod{
		PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", RestConfig: restConfig,
	})
	validateDrop := &ValidateRetinaDropMetricStep{PortForwardedRetinaPort: "10093", Direction: "unknown", Reason: IPTableRuleDrop}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: "10093", RemotePort: "10093", Endpoint: "metrics",
			RestConfig: restConfig, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateDrop},
	}
	deleteNetPol := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.NetworkPolicy), ResourceName: "deny-all", ResourceNamespace: namespace, RestConfig: restConfig,
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, RestConfig: restConfig,
	}

	wf.Add(
		flow.BatchPipe(
			flow.Pipe(createNetPol, createAgnhost, execCurl1, execCurl2).
				Timeout(utils.DefaultScenarioTimeout),
			flow.Steps(validateWithPF).
				Retry(utils.RetryWithBackoff),
			flow.Pipe(deleteNetPol, deleteAgnhost).
				When(flow.Always),
		),
	)
	return wf
}

var (
	dropCountMetricName = "networkobservability_drop_count"
	dropBytesMetricName = "networkobservability_drop_bytes"
)

const (
	IPTableRuleDrop = "IPTABLE_RULE_DROP"

	directionKey = "direction"
	reasonKey    = "reason"
)

// ValidateRetinaDropMetricStep checks that drop count and drop bytes metrics
// are present with the expected direction and reason labels.
type ValidateRetinaDropMetricStep struct {
	PortForwardedRetinaPort string
	Direction               string
	Reason                  string
}

func (v *ValidateRetinaDropMetricStep) Do(ctx context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	metric := map[string]string{
		directionKey: v.Direction,
		reasonKey:    IPTableRuleDrop,
	}

	err := prom.CheckMetric(ctx, promAddress, dropCountMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dropCountMetricName, err)
	}

	err = prom.CheckMetric(ctx, promAddress, dropBytesMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dropBytesMetricName, err)
	}
	return nil
}
