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

func addTCPScenario(kubeConfigFilePath, namespace, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
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
	validateState := &ValidateRetinaTCPStateStep{PortForwardedRetinaPort: "10093"}
	validateRemote := &ValidateRetinaTCPConnectionRemoteStep{PortForwardedRetinaPort: "10093"}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: "10093", RemotePort: "10093", Endpoint: "metrics",
			KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateState, validateRemote},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}
	deleteKapinger := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.Deployment), ResourceName: "kapinger", ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createKapinger, createAgnhost, waitKapinger, execCurl1, execCurl2).
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
		flow.Pipe(deleteAgnhost, deleteKapinger).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return wf
}



var (
	tcpStateMetricName            = "networkobservability_tcp_state"
	tcpConnectionRemoteMetricName = "networkobservability_tcp_connection_remote"
)

const (
	stateKey = "state"

	established = "ESTABLISHED"
	listen      = "LISTEN"
	timewait    = "TIME_WAIT"
)

// ValidateRetinaTCPStateStep checks that the TCP state metric exists
// for ESTABLISHED, LISTEN, and TIME_WAIT states.
type ValidateRetinaTCPStateStep struct {
	PortForwardedRetinaPort string
}

func (v *ValidateRetinaTCPStateStep) Do(ctx context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	validMetrics := []map[string]string{
		{stateKey: established},
		{stateKey: listen},
		{stateKey: timewait},
	}

	for _, metric := range validMetrics {
		err := prom.CheckMetric(ctx, promAddress, tcpStateMetricName, metric)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}
	}

	log.Printf("found metrics matching %+v\n", validMetrics)
	return nil
}

// ValidateRetinaTCPConnectionRemoteStep checks the TCP connection remote metric.
// Currently performs empty validation (no specific labels checked).
type ValidateRetinaTCPConnectionRemoteStep struct {
	PortForwardedRetinaPort string
}

func (v *ValidateRetinaTCPConnectionRemoteStep) Do(ctx context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	validMetrics := []map[string]string{}

	for _, metric := range validMetrics {
		err := prom.CheckMetric(ctx, promAddress, tcpConnectionRemoteMetricName, metric)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}
	}

	log.Printf("found metrics matching %+v\n", tcpConnectionRemoteMetricName)
	return nil
}
