// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

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

func addLatencyScenario(kubeConfigFilePath string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	validateLatency := &ValidateAPIServerLatencyStep{}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: "10093", RemotePort: "10093", Endpoint: "metrics",
			KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "k8s-app=retina",
		},
		Steps: []flow.Steper{validateLatency},
	}

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			Retry(utils.RetryWithBackoff),
	)
	return wf
}



var latencyBucketMetricName = "networkobservability_adv_node_apiserver_tcp_handshake_latency"

// ValidateAPIServerLatencyStep checks that the API server TCP handshake
// latency metric is present.
type ValidateAPIServerLatencyStep struct{}

func (v *ValidateAPIServerLatencyStep) Do(ctx context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	metric := map[string]string{}
	err := prom.CheckMetric(ctx, promAddress, latencyBucketMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify latency metrics %s: %w", latencyBucketMetricName, err)
	}

	log.Printf("found metrics matching %s\n", latencyBucketMetricName)
	return nil
}
