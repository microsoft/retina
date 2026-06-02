// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func init() {
	InitAppInsights("test", "test")
}

func TestNewAppInsightsTelemetryClient(t *testing.T) {
	require.NotPanics(t, func() { NewAppInsightsTelemetryClient("test", map[string]string{}) })
}

func TestGetEnvironmentProerties(t *testing.T) {
	properties := GetEnvironmentProperties()

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("failed to get hostname with err %v", err)
	}
	require.NotEmpty(t, properties)
	require.Exactly(
		t,
		map[string]string{
			"goversion": runtime.Version(),
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
			"numcores":  fmt.Sprintf("%d", runtime.NumCPU()),
			"hostname":  hostname,
			"podname":   os.Getenv("POD_NAME"),
		},
		properties,
	)
}

func TestNoopTelemetry(t *testing.T) {
	telemetry := NewNoopTelemetry()
	require.NotNil(t, telemetry)
	require.Equal(t, &NoopTelemetry{}, telemetry)
}

func TestNoopTelemetryStartPerf(t *testing.T) {
	telemetry := NewNoopTelemetry()
	require.NotNil(t, telemetry)
	require.Equal(t, &NoopTelemetry{}, telemetry)

	perf := telemetry.StartPerf("test")
	require.NotNil(t, perf)
	require.Equal(t, &PerformanceCounter{}, perf)
}

func TestHeartbeat(t *testing.T) {
	InitAppInsights("test", "test")
	type fields struct {
		properties map[string]string
	}
	type args struct {
		ctx   context.Context
		funcs []func() map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test heartbeat",
			fields: fields{
				properties: map[string]string{
					"test": "test",
				},
			},
			args: args{
				ctx: context.Background(),
				funcs: []func() map[string]string{
					func() map[string]string {
						return map[string]string{
							"customLabel1": "value1",
							"customLabel2": "value2",
						}
					},
				},
			},
		},
		{
			name: "test heartbeat with labels",
			fields: fields{
				properties: map[string]string{
					"test": "test",
				},
			},
			args: args{
				ctx:   context.Background(),
				funcs: []func() map[string]string{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TelemetryClient{
				RWMutex:    sync.RWMutex{},
				properties: tt.fields.properties,
				profile:    NewNoopPerfProfile(),
			}
			tr.heartbeat(tt.args.ctx, tt.args.funcs...)
		})
	}
}

func TestMetricsCardinality_nilCombinedGatherer(t *testing.T) {
	var gatherer prometheus.Gatherer

	t.Run("Combined Gatherer is nil", func(t *testing.T) {
		m, err := metricsCardinality(gatherer)
		require.Error(t, err)
		require.Equal(t, 0, m)
	})
}

type tGatherer struct {
	mf []*io_prometheus_client.MetricFamily
}

func (t *tGatherer) Gather() ([]*io_prometheus_client.MetricFamily, error) {
	return t.mf, nil
}

func TestMetricsCardinality(t *testing.T) {
	counterType := io_prometheus_client.MetricType_COUNTER
	histogramType := io_prometheus_client.MetricType_HISTOGRAM
	gaugeHistogramType := io_prometheus_client.MetricType_GAUGE_HISTOGRAM
	summaryType := io_prometheus_client.MetricType_SUMMARY

	simpleCounter := prometheus.NewCounter(prometheus.CounterOpts{ //nolint:promlinter // its a test
		Name: "test_counter",
		Help: "test counter",
	})

	histogram3buckets := prometheus.NewHistogram(prometheus.HistogramOpts{ //nolint:promlinter // its a test
		Name:    "3buckets_histogram",
		Help:    "test histogram",
		Buckets: []float64{1, 2, 3},
	})

	histogram5buckets := prometheus.NewHistogram(prometheus.HistogramOpts{ //nolint:promlinter // its a test
		Name:    "5buckets_histogram",
		Help:    "test histogram",
		Buckets: []float64{1, 2, 3, 4, 5},
	})
	summary := prometheus.NewSummary(prometheus.SummaryOpts{ //nolint:promlinter // its a test
		Name:       "test_summary",
		Help:       "test summary",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	outputCounter := io_prometheus_client.Metric{}
	err := simpleCounter.Write(&outputCounter)
	require.NoError(t, err)
	testCounter := io_prometheus_client.MetricFamily{
		Type: &counterType,
		Metric: []*io_prometheus_client.Metric{
			&outputCounter,
		},
	}

	outputHistogram3Buckets := io_prometheus_client.Metric{}
	err = histogram3buckets.Write(&outputHistogram3Buckets)
	require.NoError(t, err)
	test3BucketsHistogram := io_prometheus_client.MetricFamily{
		Type: &histogramType,
		Metric: []*io_prometheus_client.Metric{
			&outputHistogram3Buckets,
		},
	}

	outputHistogram5Buckets := io_prometheus_client.Metric{}
	err = histogram5buckets.Write(&outputHistogram5Buckets)
	require.NoError(t, err)
	test5BucketsGaugeHistogram := io_prometheus_client.MetricFamily{
		Type: &gaugeHistogramType,
		Metric: []*io_prometheus_client.Metric{
			&outputHistogram5Buckets,
		},
	}

	outputSummary := io_prometheus_client.Metric{}
	err = summary.Write(&outputSummary)
	require.NoError(t, err)
	testSummary := io_prometheus_client.MetricFamily{
		Type: &summaryType,
		Metric: []*io_prometheus_client.Metric{
			&outputSummary,
		},
	}

	testcases := []struct {
		name                string
		mf                  []*io_prometheus_client.MetricFamily
		expectedCardinality int
	}{
		{
			"Simple Counter",
			[]*io_prometheus_client.MetricFamily{&testCounter},
			1,
		},
		{
			"3 Buckets Histogram",
			[]*io_prometheus_client.MetricFamily{&test3BucketsHistogram},
			6,
		},
		{
			"5 Buckets Gauge Histogram",
			[]*io_prometheus_client.MetricFamily{&test5BucketsGaugeHistogram},
			8,
		},
		{
			"Summary",
			[]*io_prometheus_client.MetricFamily{&testSummary},
			5,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testGatherer := tGatherer{
				mf: tc.mf,
			}
			gotCardinality, err := metricsCardinality(&testGatherer)
			require.NoError(t, err)
			require.Equal(t, tc.expectedCardinality, gotCardinality)
		})
	}
}

func TestMetricsCardinality_NilHistogram(t *testing.T) {
	histogramType := io_prometheus_client.MetricType_HISTOGRAM
	gaugeHistogramType := io_prometheus_client.MetricType_GAUGE_HISTOGRAM
	summaryType := io_prometheus_client.MetricType_SUMMARY

	nilHistogram := io_prometheus_client.MetricFamily{
		Type: &histogramType,
		Metric: []*io_prometheus_client.Metric{
			{
				Histogram: nil,
			},
		},
	}
	nilGaugeHistogram := io_prometheus_client.MetricFamily{
		Type: &gaugeHistogramType,
		Metric: []*io_prometheus_client.Metric{
			{
				Histogram: nil,
			},
		},
	}
	nilSummary := io_prometheus_client.MetricFamily{
		Type: &summaryType,
		Metric: []*io_prometheus_client.Metric{
			{
				Summary: nil,
			},
		},
	}

	testGatherer := tGatherer{
		mf: []*io_prometheus_client.MetricFamily{
			&nilHistogram,
			&nilGaugeHistogram,
			&nilSummary,
		},
	}

	t.Run("Nil Histogram", func(t *testing.T) {
		m, err := metricsCardinality(&testGatherer)
		require.NoError(t, err)
		require.Equal(t, 0, m)
	})
}

func TestTelemetryClient_StopPerf(t *testing.T) {
	type fields struct {
		properties map[string]string
	}
	type args struct {
		counter *PerformanceCounter
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test stop performance",
			fields: fields{
				properties: map[string]string{
					"test": "test",
				},
			},
			args: args{
				counter: &PerformanceCounter{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TelemetryClient{
				RWMutex:    sync.RWMutex{},
				properties: tt.fields.properties,
				profile:    NewNoopPerfProfile(),
			}
			tr.StopPerf(tt.args.counter)
		})
	}
}

func TestBtoMB(t *testing.T) {
	require.Equal(t, uint64(1), bToMb(1048576))
}
