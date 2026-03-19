// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"context"
	"fmt"
	"log"

	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
)

var latencyBucketMetricName = "networkobservability_adv_node_apiserver_tcp_handshake_latency"

// ValidateAPIServerLatencyStep checks that the API server TCP handshake
// latency metric is present.
type ValidateAPIServerLatencyStep struct{}

func (v *ValidateAPIServerLatencyStep) Do(_ context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	metric := map[string]string{}
	err := prom.CheckMetric(promAddress, latencyBucketMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify latency metrics %s: %w", latencyBucketMetricName, err)
	}

	log.Printf("found metrics matching %s\n", latencyBucketMetricName)
	return nil
}
