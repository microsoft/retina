// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	v1 "github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	// Metric names
	hnsStatsName    = "adv_windows_hns_stats"
	PacketsReceived = "win_packets_recv_count"
	PacketsSent     = "win_packets_sent_count"

	// Metric descriptions
	hnsStatsDesc = "Include many different metrics from packets sent/received to closed connections"
)

var GetHNSMetadata = utils.GetHNSMetadata

type HNSMetrics struct {
	baseMetricObject
	hnsStatsMetrics metricsinit.GaugeVec
	metricName      string
}

func NewHNSMetrics(ctxOptions *api.MetricsContextOptions, l *log.ZapLogger, isLocalContext enrichmentContext) *HNSMetrics {
	l = l.Named("hns-metricsmodule")
	l.Info("Creating HNS metrics")
	return &HNSMetrics{
		baseMetricObject: newBaseMetricsObject(ctxOptions, l, isLocalContext),
	}
}

func (h *HNSMetrics) getLabels() []string {
	labels := append(h.srcCtx.getLabels(), utils.Direction)
	h.l.Info("src labels", zap.Any("labels", labels))
	return labels
}

func (h *HNSMetrics) Init(metricName string) {
	h.hnsStatsMetrics = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.AdvancedRegistry,
		hnsStatsName,
		hnsStatsDesc,
		h.getLabels()...,
	)
	h.metricName = metricName
}

func (h *HNSMetrics) ProcessFlow(flow *v1.Flow) {
	if flow == nil {
		return
	}

	// Ingress values
	ingressVal := GetHNSMetadata(flow).EndpointStats.PacketsReceived
	ingressLbls := []string{
		flow.GetIP().Source,
		flow.Source.Namespace,
		flow.Source.PodName,
		"",
		"",
		PacketsReceived,
	}
	h.hnsStatsMetrics.WithLabelValues(ingressLbls...).Set(float64(ingressVal))

	// Egress values
	egressVal := GetHNSMetadata(flow).EndpointStats.PacketsSent
	egressLbls := []string{
		flow.GetIP().Source,
		flow.Source.Namespace,
		flow.Source.PodName,
		"",
		"",
		PacketsSent,
	}
	h.hnsStatsMetrics.WithLabelValues(egressLbls...).Set(float64(egressVal))
}

func (h *HNSMetrics) Clean() {
	h.l.Info("Cleaning metric", zap.String("name", h.metricName))
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metrics.ToPrometheusType(h.hnsStatsMetrics))
}
