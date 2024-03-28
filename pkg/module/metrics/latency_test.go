// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"net"
	"testing"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/golang/mock/gomock"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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
	if lm.nodeApiServerLatency == nil {
		t.Errorf("LatencyMetrics.nodeApiServerLatency should be initialized")
	}
	if lm.noResponseMetric == nil {
		t.Errorf("LatencyMetrics.noResponseMetric should be initialized")
	}
	if lm.nodeApiServerHandshakeLatency == nil {
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
	mHist := metrics.NewMockIHistogramVec(ctrl)
	mHist.EXPECT().Observe(float64(1)).Return().Times(2)
	lm.nodeApiServerLatency = mHist

	// Set mock nodeApiServerHandshakeLatency.
	mHist2 := metrics.NewMockIHistogramVec(ctrl)
	mHist2.EXPECT().Observe(float64(1)).Return().Times(1)
	lm.nodeApiServerHandshakeLatency = mHist2

	// Test No response metric.
	c := prometheus.NewCounter(prometheus.CounterOpts{})
	mNoResponse := metrics.NewMockICounterVec(ctrl)
	// mNoResponse.EXPECT().WithLabelValues("no_response").Return(c).Times(1)
	lm.noResponseMetric = mNoResponse

	t1 := time.Now().UnixNano()
	t2 := t1 + 1*time.Millisecond.Nanoseconds()

	/*
	 * Test case 1: TCP handshake.
	 */
	// Node -> Api server.
	f1 := utils.ToFlow(t1, apiSeverIp, nodeIp, 80, 443, 6, 3, 0, 0)
	utils.AddTcpID(f1, 1234)
	utils.AddTcpFlags(f1, 1, 0, 0, 0, 0, 0)
	f1.Destination = &flow.Endpoint{
		PodName: "kubernetes-apiserver",
	}
	// Api server -> Node.
	f2 := utils.ToFlow(t2, nodeIp, apiSeverIp, 443, 80, 6, 2, 0, 0)
	utils.AddTcpID(f2, 1234)
	utils.AddTcpFlags(f2, 1, 1, 0, 0, 0, 0)
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
	utils.AddTcpFlags(f1, 1, 0, 0, 0, 0, 0)
	// Api server -> Node.
	utils.AddTcpFlags(f2, 0, 1, 0, 0, 0, 0)
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
	assert.Equal(t, value(c), float64(0), "Expected no response metric to be 0")

	// Stop.
	lm.Clean()
}

func value(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	c.Write(m)

	return m.Counter.GetValue()
}
