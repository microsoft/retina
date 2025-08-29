// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//
//nolint:all
package metrics

import (
	"strings"
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/crd/api/v1alpha1"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type TestMetrics struct {
	name            string
	opts            *v1alpha1.MetricsContextOptions
	f               *flow.Flow
	checkIsAdvance  bool
	exepectedLabels []string
	metricCall      int
	nilObj          bool
	localContext    enrichmentContext
}

func TestNewForward(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("TestNewForward")

	tt := []TestMetrics{
		{
			name:            "empty opts",
			opts:            &v1alpha1.MetricsContextOptions{},
			checkIsAdvance:  false,
			f:               &flow.Flow{},
			exepectedLabels: []string{"direction"},
			metricCall:      0,
			nilObj:          true,
		},
		{
			name: "plain opts",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "forward",
			},
			checkIsAdvance: false,
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			exepectedLabels: []string{"direction"},
			metricCall:      1,
		},
		{
			name: "plain opts with nil flow",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "forward",
			},
			checkIsAdvance:  false,
			f:               nil,
			exepectedLabels: []string{"direction"},
			metricCall:      0,
		},
		{
			name: "plain opts dropped verdict",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "forward",
			},
			checkIsAdvance: false,
			f: &flow.Flow{
				Verdict: flow.Verdict_DROPPED,
			},
			exepectedLabels: []string{"direction"},
			metricCall:      0,
		},
		{
			name: "source opts 1 without metric name",
			opts: &v1alpha1.MetricsContextOptions{
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{},
			exepectedLabels: []string{
				"direction",
			},
			metricCall: 0,
			nilObj:     true,
		},
		{
			name: "source opts 1",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "forward",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workload_kind",
				"source_workload_name",
				"source_service",
				"source_port",
			},
			metricCall: 1,
		},
		{
			name: "dest opts 1",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:        "FORWARD",
				DestinationLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"destination_ip",
				"destination_namespace",
				"destination_podname",
				"destination_workload_kind",
				"destination_workload_name",
				"destination_service",
				"destination_port",
			},
			metricCall: 1,
		},
		{
			name: "source opts with flow",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "forward",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workload_kind",
				"source_workload_name",
				"source_service",
				"source_port",
			},
			metricCall: 1,
		},
		{
			name: "drop source opts expect nil",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "drop",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
			},
			nilObj:         true,
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workload_kind",
				"source_workload_name",
				"source_service",
				"source_port",
			},
			metricCall: 1,
		},
		{
			name: "source opts with flow dropped verdict",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "forward",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_DROPPED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workload_kind",
				"source_workload_name",
				"source_service",
				"source_port",
			},
			metricCall: 0,
		},
		{
			name: "source opts with flow in local context",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "forward",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"ip",
				"namespace",
				"podname",
				"workload_kind",
				"workload_name",
				"service",
				"port",
			},
			metricCall:   1,
			localContext: LocalContext,
		},
		{
			name: "dest opts 1 with flow in local context",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "FORWARD",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict:     flow.Verdict_FORWARDED,
				Destination: &flow.Endpoint{},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"ip",
				"namespace",
				"podname",
				"workload_kind",
				"workload_name",
				"service",
				"port",
			},
			metricCall:   1,
			localContext: LocalContext,
		},
		{
			name: "src and dest opts 1 with flow in local context",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "FORWARD",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict:     flow.Verdict_FORWARDED,
				Destination: &flow.Endpoint{},
				Source:      &flow.Endpoint{},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"ip",
				"namespace",
				"podname",
				"workload_kind",
				"workload_name",
				"service",
				"port",
			},
			metricCall:   2,
			localContext: LocalContext,
		},
		{
			name: "src and dest opts 1 with flow in local context and is_reply",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:       "FORWARD",
				SourceLabels:     []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
				AdditionalLabels: []string{"is_reply"},
			},
			f: &flow.Flow{
				Verdict:     flow.Verdict_FORWARDED,
				Destination: &flow.Endpoint{},
				Source:      &flow.Endpoint{},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"ip",
				"namespace",
				"podname",
				"workload_kind",
				"workload_name",
				"service",
				"port",
				"is_reply",
			},
			metricCall:   2,
			localContext: LocalContext,
		},
	}

	for _, tc := range tt {
		for _, metricName := range []string{"forward_count", "forward_bytes"} {
			l.Info("Running test", zap.String("name", tc.name), zap.String("metricName", metricName))
			ctrl := gomock.NewController(t)

			f := NewForwardCountMetrics(tc.opts, log.Logger(), tc.localContext, false)
			if tc.nilObj {
				assert.Nil(t, f, "forward metrics should be nil Test Name: %s", tc.name)
				continue
			} else {
				assert.NotNil(t, f, "forward metrics should not be nil Test Name: %s", tc.name)
			}

			forwardMock := metricsinit.NewMockGaugeVec(ctrl) //nolint:typecheck

			f.forwardMetric = forwardMock

			testmetric := prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "testmetric",
				Help: "testmetric",
			})
			forwardMock.EXPECT().WithLabelValues(gomock.Any()).Return(testmetric).Times(tc.metricCall)
			assert.Equal(t, f.advEnable, tc.checkIsAdvance, "advance metrics options should be equal Test Name: %s", tc.name)
			assert.Equal(t, tc.exepectedLabels, f.getLabels(), "labels should be equal Test Name: %s", tc.name)

			f.metricName = metricName
			f.ProcessFlow(tc.f)
			ctrl.Finish()
		}
	}
}

func TestStandaloneForwardMetrics(t *testing.T) {
	logger, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	assert.NoError(t, err)

	ctxOptions := &api.MetricsContextOptions{
		MetricName:   "forward",
		SourceLabels: append([]string{utils.Direction}, DefaultCtxOptions()...),
	}

	forward := NewForwardCountMetrics(ctxOptions, logger, LocalContext, true)
	forward.Init(ctxOptions.MetricName)

	originalGetHNS := GetHNSMetadata
	GetHNSMetadata = func(flow *flow.Flow) *utils.HNSStatsMetadata {
		return &utils.HNSStatsMetadata{
			EndpointStats: &utils.EndpointStats{
				PacketsReceived: 42,
				PacketsSent:     99,
				BytesReceived:   42,
				BytesSent:       99,
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

	forward.ProcessFlow(testFlow)

	mfs, err := exporter.AdvancedRegistry.Gather()
	assert.NoError(t, err)
	var validMetricCount int

	for _, mf := range mfs {
		if !strings.Contains(mf.GetName(), TotalCountName) && !strings.Contains(mf.GetName(), TotalBytesName) {
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

			if labelMap["direction"] == "ingress" {
				assert.Equal(t, float64(42), m.GetGauge().GetValue())
				validMetricCount++
			} else {
				assert.Equal(t, float64(99), m.GetGauge().GetValue())
				validMetricCount++
			}
		}
	}

	assert.Equal(t, 4, validMetricCount, "Expected 4 metric samples with correct labels and values")
}
