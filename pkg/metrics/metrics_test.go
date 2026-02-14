// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"runtime"
	"testing"

	"github.com/microsoft/retina/pkg/log"
	dto "github.com/prometheus/client_model/go"
)

func TestInitialization_FirstInit(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	InitializeMetrics()

	//  All metrics should be initialized.
	objs := []interface{}{DropPacketsGauge, DropBytesGauge, ForwardBytesGauge, ForwardPacketsGauge, NodeConnectivityStatusGauge, NodeConnectivityLatencyGauge, PluginManagerFailedToReconcileCounter, BuildInfo}
	for _, obj := range objs {
		if obj == nil {
			t.Fatalf("Expected all metrics to be initialized")
		}
	}
}

func TestInitialization_MultipleInit(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Did not expect InitializeMetrics to panic on reinitialization")
		}
	}()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	InitializeMetrics()
	// Should not panic when reinitializing.
	InitializeMetrics()
}

func TestBuildInfo(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	InitializeMetrics()

	if BuildInfo == nil {
		t.Fatalf("Expected BuildInfo to be initialized")
	}

	// Verify that the build info gauge has been set with runtime information
	// Get the metric value to verify it was set correctly
	metric := &dto.Metric{}
	// Use the actual runtime values that were used to set the metric
	err := BuildInfo.WithLabelValues("unknown", runtime.GOARCH, runtime.GOOS).Write(metric)
	if err != nil {
		t.Fatalf("Failed to write metric: %v", err)
	}

	if metric.Gauge == nil {
		t.Fatalf("Expected gauge metric, got nil")
	}

	if *metric.Gauge.Value != 1 {
		t.Errorf("Expected gauge value to be 1, got %f", *metric.Gauge.Value)
	}

	t.Logf("✓ BuildInfo metric correctly set to 1 with labels: version=unknown, arch=%s, os=%s", runtime.GOARCH, runtime.GOOS)
}
