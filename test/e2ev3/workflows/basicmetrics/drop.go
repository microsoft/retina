// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"context"
	"fmt"
	"log"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
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
	execCurl1 := utils.CurlExpectFail("drop-curl-1-"+arch, &k8s.ExecInPod{
		PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	})
	execCurl2 := utils.CurlExpectFail("drop-curl-2-"+arch, &k8s.ExecInPod{
		PodNamespace: namespace, PodName: podName, Command: "curl -s -m 5 bing.com", KubeConfigFilePath: kubeConfigFilePath,
	})
	validateDrop := &ValidateRetinaDropMetricStep{PortForwardedRetinaPort: "10093", Direction: "unknown", Reason: IPTableRuleDrop}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
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

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createNetPol, createAgnhost, execCurl1, execCurl2).
			DependsOn(dependsOn).
			Timeout(utils.DefaultScenarioTimeout),
	)

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(execCurl2).
			Retry(utils.RetryWithBackoff),
	)

	// Cleanup: always runs, even if validation fails.
	wf.Add(
		flow.Pipe(deleteNetPol, deleteAgnhost).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return deleteAgnhost
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

func (v *ValidateRetinaDropMetricStep) Do(_ context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	metric := map[string]string{
		directionKey: v.Direction,
		reasonKey:    IPTableRuleDrop,
	}

	err := prom.CheckMetric(promAddress, dropCountMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dropCountMetricName, err)
	}

	err = prom.CheckMetric(promAddress, dropBytesMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dropBytesMetricName, err)
	}

	log.Printf("found metrics matching %+v\n", metric)
	return nil
}
