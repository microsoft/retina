package standalone

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/container"

	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	mm "github.com/microsoft/retina/pkg/module/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestInitModule(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	assert.NoError(t, err)

	tests := []struct {
		name             string
		wantRegistryKeys []string
		expectCleanup    bool
	}{
		{
			name: "Successful initialization",
			wantRegistryKeys: []string{
				utils.ForwardPacketsGaugeName,
				utils.ForwardBytesGaugeName,
				utils.TCPConnectionStatsName,
				utils.TCPFlagGauge,
				metrics.HNSStats,
				utils.DroppedPacketsGaugeName,
			},
			expectCleanup: false,
		},
		{
			name: "Successful cleanup after initialization",
			wantRegistryKeys: []string{
				utils.ForwardPacketsGaugeName,
				utils.ForwardBytesGaugeName,
				utils.TCPConnectionStatsName,
				utils.TCPFlagGauge,
				metrics.HNSStats,
				utils.DroppedPacketsGaugeName,
			},
			expectCleanup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			enr := enricher.NewMockEnricherInterface(ctrl)
			m := InitModule(ctx, enr)
			assert.NotNil(t, m)

			registryKeys := make([]string, 0, len(m.registry))
			for k := range m.registry {
				registryKeys = append(registryKeys, k)
			}

			for _, wantKey := range tt.wantRegistryKeys {
				assert.Contains(t, registryKeys, wantKey, "expected registry to contain %q", wantKey)
			}

			if tt.expectCleanup {
				m.Clear()
				assert.Equal(t, 0, len(m.registry))
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	enr := enricher.NewMockEnricherInterface(ctrl)
	testRing := container.NewRing(container.Capacity1)
	testRingReader := container.NewRingReader(testRing, 0)
	enr.EXPECT().ExportReader().AnyTimes().Return(testRingReader)

	originalGetHNS := mm.GetHNSMetadata
	originalRequiredMetrics := requiredMetrics
	mm.GetHNSMetadata = func(flow *flow.Flow) *utils.HNSStatsMetadata {
		return &utils.HNSStatsMetadata{
			EndpointStats: &utils.EndpointStats{
				PacketsReceived: 42,
				PacketsSent:     99,
			},
		}
	}
	requiredMetrics = []string{metrics.HNSStats}
	defer func() {
		mm.GetHNSMetadata = originalGetHNS
		requiredMetrics = originalRequiredMetrics
	}()

	m := InitModule(ctx, enr)
	assert.NotNil(t, m)

	m.Reconcile(ctx)

	testFlow := &flow.Flow{
		IP: &flow.IP{Source: "1.1.1.1"},
		Source: &flow.Endpoint{
			Namespace: "default",
			PodName:   "test-pod",
		},
	}

	event := &v1.Event{Event: testFlow}
	for i := 0; i < 2; i++ {
		testRing.Write(event)
		time.Sleep(25 * time.Millisecond)
	}

	mfs, err := exporter.AdvancedRegistry.Gather()
	assert.NoError(t, err)
	var validMetricCount int

	for _, mf := range mfs {
		if !strings.Contains(mf.GetName(), metrics.HNSStats) {
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
			assert.Equal(t, "", labelMap["workload_kind"])
			assert.Equal(t, "", labelMap["workload_name"])

			if labelMap["direction"] == mm.PacketsReceived {
				assert.Equal(t, float64(42), m.GetGauge().GetValue())
				validMetricCount++
			} else {
				assert.Equal(t, float64(99), m.GetGauge().GetValue())
				validMetricCount++
			}
		}
	}
	assert.Equal(t, 2, validMetricCount, "Expected 2 metric samples with correct labels and values")
}
