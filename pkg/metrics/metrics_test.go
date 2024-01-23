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
	objs := []interface{}{DropCounter, DropBytesCounter, ForwardBytesCounter, ForwardCounter, NodeConnectivityStatusGauge, NodeConnectivityLatencyGauge, PluginManagerFailedToReconcileCounter}
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
