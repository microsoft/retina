// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"strings"
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHNSMetrics(t *testing.T) {
	logger, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	ctxOptions := &api.MetricsContextOptions{
		MetricName:   "hns",
		SourceLabels: append(DefaultCtxOptions(), utils.Direction),
	}

	hns := NewHNSMetrics(ctxOptions, logger, LocalContext)
	hns.Init(ctxOptions.MetricName)

	originalGetHNS := GetHNSMetadata
	GetHNSMetadata = func(_ *flow.Flow) *utils.HNSStatsMetadata {
		return &utils.HNSStatsMetadata{
			EndpointStats: &utils.EndpointStats{
				PacketsReceived: 42,
				PacketsSent:     99,
			},
		}
	}
	defer func() { GetHNSMetadata = originalGetHNS }()

	testFlow := &flow.Flow{
		IP: &flow.IP{Source: "1.1.1.1"},
		Source: &flow.Endpoint{
			Namespace: "default",
			PodName:   "test-pod",
		},
	}

	hns.ProcessFlow(testFlow)

	mfs, err := exporter.AdvancedRegistry.Gather()
	require.NoError(t, err)
	var validMetricCount int

	for _, mf := range mfs {
		if !strings.Contains(mf.GetName(), hnsStatsName) {
			continue
		}
		t.Logf("Metric Family: %s", mf.GetName())

		for _, m := range mf.GetMetric() {
			labelMap := map[string]string{}
			for _, label := range m.GetLabel() {
				labelMap[label.GetName()] = label.GetValue()
			}
			assert.Equal(t, "1.1.1.1", labelMap["ip"])
			assert.Equal(t, "default", labelMap["namespace"])
			assert.Equal(t, "test-pod", labelMap["podname"])
			assert.Empty(t, labelMap["workload_kind"])
			assert.Empty(t, labelMap["workload_name"])

			if labelMap["direction"] == PacketsReceived {
				assert.InEpsilon(t, float64(42), m.GetGauge().GetValue(), 0.001)
				validMetricCount++
			} else {
				assert.InEpsilon(t, float64(99), m.GetGauge().GetValue(), 0.001)
				validMetricCount++
			}
		}
	}
	assert.Equal(t, 2, validMetricCount, "Expected 2 metric samples with correct labels and values")
}
