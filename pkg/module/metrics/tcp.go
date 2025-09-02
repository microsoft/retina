// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"strings"

	v1 "github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	// Metric names
	TCPConnectionStatsName = "adv_tcp_connection_stats"

	// Metric descriptions
	TCPConnectionStatsGaugeDescription = "TCP connection statistics"
)

type TCPConnectionMetrics struct {
	baseMetricObject
	tcpConnStatsGauge metricsinit.GaugeVec
	metricName        string
}

func NewTCPConnectionMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext) *TCPConnectionMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "tcp_connection") {
		return nil
	}

	fl = fl.Named("tcpconnection-metricsmodule")
	fl.Info("Creating TCP Connection Stats metrics", zap.Any("options", ctxOptions))
	return &TCPConnectionMetrics{
		baseMetricObject: newBaseMetricsObject(ctxOptions, fl, isLocalContext),
	}
}

func (t *TCPConnectionMetrics) Init(metricName string) {
	t.tcpConnStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.AdvancedRegistry,
		TCPConnectionStatsName,
		TCPConnectionStatsGaugeDescription,
		t.getLabels()...,
	)
	t.metricName = metricName
}

func (t *TCPConnectionMetrics) getLabels() []string {
	labels := []string{
		utils.StatName,
	}

	if t.srcCtx != nil {
		labels = append(labels, t.srcCtx.getLabels()...)
		t.l.Info("src labels", zap.Any("labels", labels))
	}

	if t.dstCtx != nil {
		labels = append(labels, t.dstCtx.getLabels()...)
		t.l.Info("dst labels", zap.Any("labels", labels))
	}

	return labels
}

func (t *TCPConnectionMetrics) ProcessFlow(flow *v1.Flow) {
	if flow == nil || GetHNSMetadata(flow).GetVfpPortStatsData() == nil {
		return
	}

	// label values
	lbls := []string{
		flow.GetIP().GetSource(),
		flow.GetSource().GetNamespace(),
		flow.GetSource().GetPodName(),
		"",
		"",
	}

	// Get the TCP connection counters
	tcpStats := GetHNSMetadata(flow).GetVfpPortStatsData().GetIn().GetTcpCounters().GetConnectionCounters()

	t.tcpConnStatsGauge.WithLabelValues(append([]string{utils.ResetCount}, lbls...)...).Set(float64(tcpStats.GetResetCount()))
	t.tcpConnStatsGauge.WithLabelValues(append([]string{utils.ClosedFin}, lbls...)...).Set(float64(tcpStats.GetClosedFinCount()))
	t.tcpConnStatsGauge.WithLabelValues(append([]string{utils.ResetSyn}, lbls...)...).Set(float64(tcpStats.GetResetSynCount()))
	t.tcpConnStatsGauge.WithLabelValues(append([]string{utils.TcpHalfOpenTimeouts}, lbls...)...).Set(float64(tcpStats.GetTcpHalfOpenTimeoutsCount()))
	t.tcpConnStatsGauge.WithLabelValues(append([]string{utils.Verified}, lbls...)...).Set(float64(tcpStats.GetVerifiedCount()))
	t.tcpConnStatsGauge.WithLabelValues(append([]string{utils.TimedOutCount}, lbls...)...).Set(float64(tcpStats.GetTimedOutCount()))
	t.tcpConnStatsGauge.WithLabelValues(append([]string{utils.TimeWaitExpiredCount}, lbls...)...).Set(float64(tcpStats.GetTimeWaitExpiredCount()))
}

func (t *TCPConnectionMetrics) Clean() {
	t.l.Info("Cleaning metric", zap.String("name", t.metricName))
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(t.tcpConnStatsGauge))
}
