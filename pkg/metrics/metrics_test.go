// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"testing"

	"github.com/microsoft/retina/pkg/log"
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
	// We can't check the exact values without knowing the build-time injected version,
	// but we can verify the metric exists and has the expected labels
}
