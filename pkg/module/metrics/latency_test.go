// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"net"
	"testing"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestNewLatencyMetrics(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test")

	options := []*api.MetricsContextOptions{
		nil,
		{
			MetricName: "forward",
		},
		{
			MetricName: "node_apiserver_latency",
		},
		{
			MetricName: "node_apiserver_handshake_latency",
		},
		{
			MetricName: "node_apiserver_no_response",
		},
	}
	for _, option := range options[:2] {
		if NewLatencyMetrics(option, l, "") != nil {
			t.Errorf("NewLatencyMetrics(%v) should return nil", option)
		}
	}

	var lm *LatencyMetrics
	for _, option := range options[2:] {
		lm = NewLatencyMetrics(option, l, "")
	}
	if lm.nodeAPIServerLatency == nil {
		t.Errorf("LatencyMetrics.nodeApiServerLatency should be initialized")
	}
	if lm.noResponseMetric == nil {
		t.Errorf("LatencyMetrics.noResponseMetric should be initialized")
	}
	if lm.nodeAPIServerHandshakeLatency == nil {
		t.Errorf("LatencyMetrics.nodeApiServerHandshakeLatency should be initialized")
	}

	// Stop.
	lm.Clean()
}

func TestInit(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test")

	exporter.AdvancedRegistry = prometheus.NewRegistry()
	lm := &LatencyMetrics{l: l}

	lm.Init("latency")

	if lm.cache == nil {
		t.Errorf("LatencyMetrics.cache should be initialized")
	}

	// Stop.
	lm.Clean()
}

func TestProcessFlow(t *testing.T) {
	apiSeverIp := net.IPv4(1, 1, 1, 1)
	nodeIp := net.IPv4(2, 2, 2, 2)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exporter.AdvancedRegistry = prometheus.NewRegistry()
	lm := &LatencyMetrics{l: l}

	lm.Init("latency")

	// Set mock apiServerIps.
	lm.apiServerIps[apiSeverIp.String()] = struct{}{}

	// Set mock nodeApiServerLatency.
	mHist := metrics.NewMockHistogram(ctrl)
	mHist.EXPECT().Observe(float64(1)).Return().Times(2)
	lm.nodeAPIServerLatency = mHist

	// Set mock nodeApiServerHandshakeLatency.
	mHist2 := metrics.NewMockHistogram(ctrl)
	mHist2.EXPECT().Observe(float64(1)).Return().Times(1)
	lm.nodeAPIServerHandshakeLatency = mHist2

	// Test No response metric.
	c := prometheus.NewCounter(prometheus.CounterOpts{})
	mNoResponse := metrics.NewMockCounterVec(ctrl)
	mNoResponse.EXPECT().WithLabelValues("no_response").Return(c).Times(1)
	lm.noResponseMetric = mNoResponse

	t1 := time.Now().UnixNano()
	t2 := t1 + 1*time.Millisecond.Nanoseconds()

	/*
	 * Test case 1: TCP handshake.
	 */
	// Node -> Api server.
	f1 := utils.ToFlow(l, t1, apiSeverIp, nodeIp, 80, 443, 6, 3, 0)
	ext1 := utils.NewExtensions()
	utils.AddTCPID(ext1, 1234)
	utils.AddTCPFlags(f1, 1, 0, 0, 0, 0, 0, 0, 0, 0)
	utils.SetExtensions(f1, ext1)
	f1.Destination = &flow.Endpoint{
		PodName: "kubernetes-apiserver",
	}

	// Api server -> Node.
	f2 := utils.ToFlow(l, t2, nodeIp, apiSeverIp, 443, 80, 6, 2, 0)
	ext2 := utils.NewExtensions()
	utils.AddTCPID(ext2, 1234)
	utils.AddTCPFlags(f2, 1, 1, 0, 0, 0, 0, 0, 0, 0)
	utils.SetExtensions(f2, ext2)
	f2.Source = &flow.Endpoint{
		PodName: "kubernetes-apiserver",
	}
	// Process flow.
	lm.ProcessFlow(f1)
	lm.ProcessFlow(f2)

	/*
	 * Test case 2: Existing TCP connection.
	 */
	// Node -> Api server.
	utils.AddTCPFlags(f1, 1, 0, 0, 0, 0, 0, 0, 0, 0)
	// Api server -> Node.
	utils.AddTCPFlags(f2, 0, 1, 0, 0, 0, 0, 0, 0, 0)
	// Process flow.
	lm.ProcessFlow(f1)
	lm.ProcessFlow(f2)

	/*
	 * Test case 3: No reply from apiserver.
	 */
	lm.ProcessFlow(f1)
	// Sleep for TTL.
	time.Sleep(1 * time.Second)
	// Check dropped packet.
	assert.Equal(t, value(c), float64(1), "Expected no response metric to be 1")

	// Stop.
	lm.Clean()
}

// TestCalculateLatencyAcrossSecondBoundary is a regression test for a bug where only
// the sub-second part of the flow timestamp (Timestamp.Nanos) was used to compute
// latency. When a request and its reply were observed in different seconds, the
// result was a large negative value (e.g. -999ms instead of 1ms).
func TestCalculateLatencyAcrossSecondBoundary(t *testing.T) {
	apiServerIP := net.IPv4(1, 1, 1, 1)
	nodeIP := net.IPv4(2, 2, 2, 2)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exporter.AdvancedRegistry = prometheus.NewRegistry()
	lm := &LatencyMetrics{l: l}
	lm.Init("latency")
	defer lm.Clean()

	lm.apiServerIps[apiServerIP.String()] = struct{}{}

	// Exactly one observation of 1ms is expected.
	mHist := metrics.NewMockHistogram(ctrl)
	mHist.EXPECT().Observe(float64(1)).Return().Times(1)
	lm.nodeAPIServerLatency = mHist

	// The request leaves 0.5ms before a second boundary and the reply arrives 1ms
	// later, i.e. 0.5ms into the next second.
	t1 := time.Date(2026, time.January, 1, 12, 0, 0, 999_500_000, time.UTC).UnixNano()
	t2 := t1 + time.Millisecond.Nanoseconds()

	// Node -> Api server.
	f1 := utils.ToFlow(l, t1, apiServerIP, nodeIP, 80, 443, 6, 3, 0)
	ext1 := utils.NewExtensions()
	utils.AddTCPID(ext1, 1234)
	utils.SetExtensions(f1, ext1)

	// Api server -> Node.
	f2 := utils.ToFlow(l, t2, nodeIP, apiServerIP, 443, 80, 6, 2, 0)
	ext2 := utils.NewExtensions()
	utils.AddTCPID(ext2, 1234)
	utils.SetExtensions(f2, ext2)

	lm.ProcessFlow(f1)
	lm.ProcessFlow(f2)
}

func value(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	c.Write(m)

	return m.Counter.GetValue()
}
