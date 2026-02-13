// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"strings"
	"time"

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
	baseMetricInterface
	tcpRetransMetrics metricsinit.GaugeVec
}

func NewTCPRetransMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext, ttl time.Duration) *TCPRetransMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "retrans") {
		return nil
	}

	fl = fl.Named("tcpretrans-metricsmodule")
	fl.Info("Creating TCP retransmit count metrics", zap.Any("options", ctxOptions))
	t := &TCPRetransMetrics{}
	t.baseMetricInterface = newBaseMetricsObject(ctxOptions, fl, isLocalContext, t.expire, ttl)
	return t
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
	if t.sourceCtx() != nil {
		labels = append(labels, t.sourceCtx().getLabels()...)
		t.getLogger().Info("src labels", zap.Any("labels", labels))
	}

	if t.destinationCtx() != nil {
		labels = append(labels, t.destinationCtx().getLabels()...)
		t.getLogger().Info("dst labels", zap.Any("labels", labels))
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
	if t.sourceCtx() != nil {
		srcLabels := t.sourceCtx().getValues(flow)
		if len(srcLabels) > 0 {
			labels = append(labels, srcLabels...)
		}
	}

	if t.destinationCtx() != nil {
		dstLabels := t.destinationCtx().getValues(flow)
		if len(dstLabels) > 0 {
			labels = append(labels, dstLabels...)
		}
	}

	t.update(labels)
}

func (t *TCPRetransMetrics) processLocalCtxFlow(flow *v1.Flow) {
	labelValuesMap := t.sourceCtx().getLocalCtxValues(flow)
	if labelValuesMap == nil {
		return
	}

	if len(labelValuesMap[ingress]) > 0 {
		labels := append([]string{ingress}, labelValuesMap[ingress]...)
		t.update(labels)
		t.getLogger().Debug("tcp retransmission count metric in INGRESS in local ctx", zap.Any("labels", labels))
	}

	if len(labelValuesMap[egress]) > 0 {
		labels := append([]string{egress}, labelValuesMap[egress]...)
		t.update(labels)
		t.getLogger().Debug("tcp retransmission count metric in EGRESS in local ctx", zap.Any("labels", labels))
	}
}

func (t *TCPRetransMetrics) expire(labels []string) bool {
	var d bool
	if t.tcpRetransMetrics != nil {
		d = t.tcpRetransMetrics.DeleteLabelValues(labels...)
		if d {
			metricsinit.MetricsExpiredCounter.WithLabelValues(TCPRetransCountName).Inc()
		}
	}
	return d
}

func (t *TCPRetransMetrics) update(labels []string) {
	t.tcpRetransMetrics.WithLabelValues(labels...).Inc()
	t.updated(labels)
}

func (t *TCPRetransMetrics) Clean() {
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(t.tcpRetransMetrics))
	t.clean()
}
