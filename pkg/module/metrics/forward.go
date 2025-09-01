// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"slices"
	"strconv"
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
	TotalCountName = "adv_forward_count"

	// TODO remove bytes as it is not being populated
	TotalBytesName = "adv_forward_bytes"

	TotalCountDesc = "Total number of forwarded packets"
	TotalBytesDesc = "Total number of forwarded bytes"
)

type ForwardMetrics struct {
	baseMetricObject
	forwardMetric metricsinit.GaugeVec
	// bytesMetric metricsinit.IGaugeVec
	metricName string
}

func NewForwardCountMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext) *ForwardMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "forward") {
		return nil
	}

	l := fl.Named("forward-metricsmodule")
	l.Info("Creating forward count metrics", zap.Any("options", ctxOptions))
	return &ForwardMetrics{
		baseMetricObject: newBaseMetricsObject(ctxOptions, fl, isLocalContext),
	}
}

func (f *ForwardMetrics) Init(metricName string) {
	switch metricName {
	case utils.ForwardPacketsGaugeName:
		f.forwardMetric = exporter.CreatePrometheusGaugeVecForMetric(
			exporter.AdvancedRegistry,
			TotalCountName,
			TotalCountDesc,
			f.getLabels()...)
	case utils.ForwardBytesGaugeName:
		f.forwardMetric = exporter.CreatePrometheusGaugeVecForMetric(
			exporter.AdvancedRegistry,
			TotalBytesName,
			TotalBytesDesc,
			f.getLabels()...)
	default:
		f.l.Error("unknown metric name", zap.String("name", metricName))
	}
	f.metricName = metricName
}

func (f *ForwardMetrics) getLabels() []string {
	labels := []string{
		utils.Direction,
	}

	if !f.advEnable {
		return labels
	}

	if f.srcCtx != nil {
		labels = append(labels, f.srcCtx.getLabels()...)
		f.l.Info("src labels", zap.Any("labels", labels))
	}

	if f.dstCtx != nil {
		labels = append(labels, f.dstCtx.getLabels()...)
		f.l.Info("dst labels", zap.Any("labels", labels))
	}

	if slices.Contains(f.ctxOptions.AdditionalLabels, utils.IsReply) {
		labels = append(labels, utils.IsReply)
	}

	return labels
}

func (f *ForwardMetrics) Clean() {
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(f.forwardMetric))
}

// TODO: update ProcessFlow with bytes metrics. We are only accounting for count.
// bytes metrics needs some additional work in ebpf and in this func to get the skb length
func (f *ForwardMetrics) ProcessFlow(flow *v1.Flow) {
	// Flow does not have bytes section at the moment,
	// so we will update only packet count
	if flow == nil {
		return
	}

	if flow.Verdict != v1.Verdict_FORWARDED {
		return
	}

	if f.isLocalContext() {
		// when localcontext is enabled, we do not need the context options for both src and dst
		// metrics aggregation will be on a single pod basis and not the src/dst pod combination basis.
		f.processLocalCtxFlow(flow)
		return
	}

	labels := []string{
		flow.TrafficDirection.String(),
	}

	if !f.advEnable {
		f.update(flow, labels)
		return
	}

	if f.srcCtx != nil {
		srcLabels := f.srcCtx.getValues(flow)
		if len(srcLabels) > 0 {
			labels = append(labels, srcLabels...)
		}
	}

	if f.dstCtx != nil {
		dstLabel := f.dstCtx.getValues(flow)
		if len(dstLabel) > 0 {
			labels = append(labels, dstLabel...)
		}
	}

	if slices.Contains(f.ctxOptions.AdditionalLabels, utils.IsReply) {
		labels = append(labels, strconv.FormatBool(flow.GetIsReply().GetValue()))
	}

	f.update(flow, labels)
	f.l.Debug("forward count metric is added", zap.Any("labels", labels))
}

func (f *ForwardMetrics) processLocalCtxFlow(flow *v1.Flow) {
	labelValuesMap := f.srcCtx.getLocalCtxValues(flow)
	if labelValuesMap == nil {
		return
	}
	// Ingress values.
	if len(labelValuesMap[ingress]) > 0 {
		labels := append([]string{ingress}, labelValuesMap[ingress]...)
		f.update(flow, labels)
		f.l.Debug("forward count metric in INGRESS in local ctx", zap.Any("labels", labels))
	}

	// Egress values.
	if len(labelValuesMap[egress]) > 0 {
		labels := append([]string{egress}, labelValuesMap[egress]...)
		f.update(flow, labels)
		f.l.Debug("forward count metric in EGRESS in local ctx", zap.Any("labels", labels))
	}
}

func (f *ForwardMetrics) update(fl *v1.Flow, labels []string) {
	switch f.metricName {
	case utils.ForwardPacketsGaugeName:
		f.forwardMetric.WithLabelValues(labels...).Add(float64(utils.PreviouslyObservedPackets(fl) + 1))
	case utils.ForwardBytesGaugeName:
		f.forwardMetric.WithLabelValues(labels...).Add(float64(utils.PacketSize(fl) + utils.PreviouslyObservedBytes(fl)))
	}
}
