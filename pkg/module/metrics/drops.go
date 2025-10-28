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
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	TotalDropCountName = "adv_drop_count"
	TotalDropBytesName = "adv_drop_bytes"

	TotalDropCountDesc = "Total number of dropped packets"
	TotalDropBytesDesc = "Total number of dropped bytes"
)

type DropCountMetrics struct {
	baseMetricInterface
	dropMetric metrics.GaugeVec
	metricName string
}

func NewDropCountMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext, ttl time.Duration) *DropCountMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "drop") {
		return nil
	}

	fl = fl.Named("dropreason-metricsmodule")
	fl.Info("Creating drop count metrics", zap.Any("options", ctxOptions))
	d := &DropCountMetrics{}
	d.baseMetricInterface = newBaseMetricsObject(ctxOptions, fl, isLocalContext, d.expire, ttl)
	return d
}

func (d *DropCountMetrics) Init(metricName string) {
	switch metricName {
	case utils.DroppedPacketsGaugeName:
		d.dropMetric = exporter.CreatePrometheusGaugeVecForMetric(
			exporter.AdvancedRegistry,
			TotalDropCountName,
			TotalDropCountDesc,
			d.getLabels()...)
	case utils.DropBytesGaugeName:
		d.dropMetric = exporter.CreatePrometheusGaugeVecForMetric(
			exporter.AdvancedRegistry,
			TotalDropBytesName,
			TotalDropBytesDesc,
			d.getLabels()...)
	default:
		d.getLogger().Error("unknown metric name", zap.String("metricName", metricName))
	}
	d.metricName = metricName
}

func (d *DropCountMetrics) getLabels() []string {
	labels := []string{
		utils.Reason,
		utils.Direction,
	}

	if d.sourceCtx() != nil {
		labels = append(labels, d.sourceCtx().getLabels()...)
		d.getLogger().Info("src labels", zap.Any("labels", labels))
	}

	if d.destinationCtx() != nil {
		labels = append(labels, d.destinationCtx().getLabels()...)
		d.getLogger().Info("dst labels", zap.Any("labels", labels))
	}

	// No additional context options

	return labels
}

func (d *DropCountMetrics) Clean() {
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metrics.ToPrometheusType(d.dropMetric))
	d.clean()
}

// TODO: update ProcessFlow with bytes metrics. We are only accounting for count.
// bytes metrics needs some additional work in ebpf and in this func to get the skb length
func (d *DropCountMetrics) ProcessFlow(flow *v1.Flow) {
	// Flow does not have bytes section at the moment,
	// so we will update only packet count
	if flow == nil {
		return
	}

	if flow.Verdict != v1.Verdict_DROPPED {
		return
	}

	if d.isLocalContext() {
		// when localcontext is enabled, we do not need the context options for both src and dst
		// metrics aggregation will be on a single pod basis and not the src/dst pod combination basis.
		d.processLocalCtxFlow(flow)
		return
	}

	labels := []string{
		utils.DropReasonDescription(flow),
		flow.TrafficDirection.String(),
	}

	if !d.isAdvanced() {
		d.update(flow, labels)
		return
	}

	if d.sourceCtx() != nil {
		srcLabels := d.sourceCtx().getValues(flow)
		if len(srcLabels) > 0 {
			labels = append(labels, srcLabels...)
		}
	}

	if d.destinationCtx() != nil {
		dstLabel := d.destinationCtx().getValues(flow)
		if len(dstLabel) > 0 {
			labels = append(labels, dstLabel...)
		}
	}

	// No additional context options

	d.update(flow, labels)
	d.getLogger().Debug("drop count metric is added", zap.Any("labels", labels))
}

func (d *DropCountMetrics) processLocalCtxFlow(flow *v1.Flow) {
	labelValuesMap := d.sourceCtx().getLocalCtxValues(flow)
	if labelValuesMap == nil {
		return
	}
	dropReason := utils.DropReasonDescription(flow)

	// Ingress values
	if l := len(labelValuesMap[ingress]); l > 0 {
		labels := make([]string, 0, l+2)
		labels = append(labels, dropReason, ingress)
		labels = append(labels, labelValuesMap[ingress]...)
		d.update(flow, labels)
		d.getLogger().Debug("drop count metric is added in INGRESS in local ctx", zap.Any("labels", labels))
	}

	if l := len(labelValuesMap[egress]); l > 0 {
		labels := make([]string, 0, l+2)
		labels = append(labels, dropReason, egress)
		labels = append(labels, labelValuesMap[egress]...)
		d.update(flow, labels)
		d.getLogger().Debug("drop count metric is added in EGRESS in local ctx", zap.Any("labels", labels))
	}
}

func (d *DropCountMetrics) expire(labels []string) bool {
	var del bool
	if d.dropMetric != nil {
		del = d.dropMetric.DeleteLabelValues(labels...)
		if del {
			metrics.MetricsExpiredCounter.WithLabelValues(d.metricName).Inc()
		}
	}
	return del
}

func (d *DropCountMetrics) update(fl *v1.Flow, labels []string) {
	var updated bool
	switch d.metricName {
	case utils.DroppedPacketsGaugeName:
		updated = true
		d.dropMetric.WithLabelValues(labels...).Inc()
	case utils.DropBytesGaugeName:
		updated = true
		d.dropMetric.WithLabelValues(labels...).Add(float64(utils.PacketSize(fl)))
	}
	if updated {
		d.updated(labels)
	}
}
