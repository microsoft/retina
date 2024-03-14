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
	TCPRetransCountName = "adv_tcpretrans_count"

	// Metric descriptions
	TCPRetransCountDesc = "Total number of TCP retransmitted packets"
)

type TCPRetransMetrics struct {
	baseMetricObject
	tcpRetransMetrics metricsinit.IGaugeVec
}

func NewTCPRetransMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext) *TCPRetransMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "retrans") {
		return nil
	}

	fl = fl.Named("tcpretrans-metricsmodule")
	fl.Info("Creating TCP retransmit count metrics", zap.Any("options", ctxOptions))
	return &TCPRetransMetrics{
		baseMetricObject: newBaseMetricsObject(ctxOptions, fl, isLocalContext),
	}
}

func (t *TCPRetransMetrics) Init(metricName string) {
	// only 1 metric. No need to check metric name which is already validated.
	t.tcpRetransMetrics = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.AdvancedRegistry,
		TCPRetransCountName,
		TCPRetransCountDesc,
		t.getLabels()...,
	)
}

func (t *TCPRetransMetrics) getLabels() []string {
	labels := []string{utils.Direction}
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

func (t *TCPRetransMetrics) ProcessFlow(flow *v1.Flow) {
	if flow == nil {
		return
	}

	if flow.Verdict != utils.Verdict_RETRANSMISSION {
		return
	}

	if t.isLocalContext() {
		// when localcontext is enabled, we do not need the context options for both src and dst
		// metrics aggregation will be on a single pod basis and not the src/dst pod combination basis.
		t.processLocalCtxFlow(flow)
		return
	}

	labels := []string{flow.TrafficDirection.String()}
	if t.srcCtx != nil {
		srcLabels := t.srcCtx.getValues(flow)
		if len(srcLabels) > 0 {
			labels = append(labels, srcLabels...)
		}
	}

	if t.dstCtx != nil {
		dstLabels := t.dstCtx.getValues(flow)
		if len(dstLabels) > 0 {
			labels = append(labels, dstLabels...)
		}
	}

	t.tcpRetransMetrics.WithLabelValues(labels...).Inc()
}

func (t *TCPRetransMetrics) processLocalCtxFlow(flow *v1.Flow) {
	labelValuesMap := t.srcCtx.getLocalCtxValues(flow)
	if labelValuesMap == nil {
		return
	}

	if len(labelValuesMap[ingress]) > 0 {
		labels := append([]string{ingress}, labelValuesMap[ingress]...)
		t.tcpRetransMetrics.WithLabelValues(labels...).Inc()
		t.l.Debug("tcp retransmission count metric in INGRESS in local ctx", zap.Any("labels", labels))
	}

	if len(labelValuesMap[egress]) > 0 {
		labels := append([]string{egress}, labelValuesMap[egress]...)
		t.tcpRetransMetrics.WithLabelValues(labels...).Inc()
		t.l.Debug("tcp retransmission count metric in EGRESS in local ctx", zap.Any("labels", labels))
	}
}

func (t *TCPRetransMetrics) Clean() {
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(t.tcpRetransMetrics))
}
