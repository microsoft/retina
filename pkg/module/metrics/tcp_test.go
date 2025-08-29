package metrics

import (
	"strings"
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestTCPConnectionMetrics(t *testing.T) {
	logger, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	assert.NoError(t, err)

	ctxOptions := &v1alpha1.MetricsContextOptions{
		MetricName:   TCPConnectionStatsName,
		SourceLabels: append([]string{utils.StatName}, DefaultCtxOptions()...),
	}

	tcp := NewTCPConnectionMetrics(ctxOptions, logger, LocalContext)
	tcp.Init(ctxOptions.MetricName)

	originalGetHNS := GetHNSMetadata
	GetHNSMetadata = func(flow *flow.Flow) *utils.HNSStatsMetadata {
		return &utils.HNSStatsMetadata{
			VfpPortStatsData: &utils.VfpPortStatsData{
				In: &utils.VfpDirectedPortCounters{
					TcpCounters: &utils.VfpTcpStats{
						ConnectionCounters: &utils.VfpTcpConnectionStats{
							VerifiedCount:            10,
							TimedOutCount:            20,
							ResetCount:               30,
							ResetSynCount:            40,
							ClosedFinCount:           50,
							TcpHalfOpenTimeoutsCount: 60,
							TimeWaitExpiredCount:     70,
						},
					},
				},
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

	tcp.ProcessFlow(testFlow)

	mfs, err := exporter.AdvancedRegistry.Gather()
	assert.NoError(t, err)
	var validMetricCount int

	for _, mf := range mfs {
		if !strings.Contains(mf.GetName(), TCPConnectionStatsName) {
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

			if labelMap[utils.StatName] == utils.Verified {
				assert.Equal(t, float64(10), m.GetGauge().GetValue())
				validMetricCount++
			} else if labelMap[utils.StatName] == utils.TimedOutCount {
				assert.Equal(t, float64(20), m.GetGauge().GetValue())
				validMetricCount++
			} else if labelMap[utils.StatName] == utils.ResetCount {
				assert.Equal(t, float64(30), m.GetGauge().GetValue())
				validMetricCount++
			} else if labelMap[utils.StatName] == utils.ResetSyn {
				assert.Equal(t, float64(40), m.GetGauge().GetValue())
				validMetricCount++
			} else if labelMap[utils.StatName] == utils.ClosedFin {
				assert.Equal(t, float64(50), m.GetGauge().GetValue())
				validMetricCount++
			} else if labelMap[utils.StatName] == utils.TcpHalfOpenTimeouts {
				assert.Equal(t, float64(60), m.GetGauge().GetValue())
				validMetricCount++
			} else if labelMap[utils.StatName] == utils.TimeWaitExpiredCount {
				assert.Equal(t, float64(70), m.GetGauge().GetValue())
				validMetricCount++
			}
		}
	}

	assert.Equal(t, 7, validMetricCount, "Expected 7 metric samples with correct labels and values")
}
