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
	TCPFlagsCountName = "adv_tcpflags_count"

	// Metric descriptions
	TCPFlagsCountDesc = "Total number of packets by TCP flag"
)

type TCPMetrics struct {
	baseMetricInterface
	tcpFlagsMetrics metricsinit.GaugeVec
}

func NewTCPMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext, ttl time.Duration) *TCPMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "flag") {
		return nil
	}

	fl = fl.Named("tcpflags-metricsmodule")
	fl.Info("Creating TCP Flags count metrics", zap.Any("options", ctxOptions))
	t := &TCPMetrics{}
	t.baseMetricInterface = newBaseMetricsObject(ctxOptions, fl, isLocalContext, t.expire, ttl)
	return t
}

func (t *TCPMetrics) Init(metricName string) {
	// only 1 metric. No need to check metric name which is already validated.
	t.tcpFlagsMetrics = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.AdvancedRegistry,
		TCPFlagsCountName,
		TCPFlagsCountDesc,
		t.getLabels()...,
	)
}

func (t *TCPMetrics) getLabels() []string {
	labels := []string{
		utils.Flag,
	}
	if t.sourceCtx() != nil {
		labels = append(labels, t.sourceCtx().getLabels()...)
	}

	if t.destinationCtx() != nil {
		labels = append(labels, t.destinationCtx().getLabels()...)
	}

	return labels
}

func combineFlagsWithPrevious(flags []string, flow *v1.Flow) map[string]uint32 {
	var combinedFlags map[string]uint32

	previous := utils.PreviouslyObservedTCPFlags(flow)
	if previous != nil {
		combinedFlags = previous
	} else {
		combinedFlags = map[string]uint32{}
	}

	for _, flag := range flags {
		if _, ok := combinedFlags[flag]; !ok {
			combinedFlags[flag] = 1
		} else {
			combinedFlags[flag]++
		}
	}

	return combinedFlags
}

func (t *TCPMetrics) ProcessFlow(flow *v1.Flow) {
	if flow == nil {
		return
	}

	if flow.Verdict != v1.Verdict_FORWARDED {
		return
	}

	tcp := flow.L4.GetTCP()
	if tcp == nil {
		return
	}

	flags := t.getFlagValues(tcp.GetFlags())
	if len(flags) == 0 {
		return
	}

	if t.isLocalContext() {
		// when localcontext is enabled, we do not need the context options for both src and dst
		// metrics aggregation will be on a single pod basis and not the src/dst pod combination basis.
		t.processLocalCtxFlow(flow, flags)
		return
	}

	var srcLabels, dstLabels []string
	if t.sourceCtx() != nil {
		srcLabels = t.sourceCtx().getValues(flow)
	}

	if t.destinationCtx() != nil {
		dstLabels = t.destinationCtx().getValues(flow)
	}

	for flag, count := range combineFlagsWithPrevious(flags, flow) {
		labels := append([]string{flag}, srcLabels...)
		labels = append(labels, dstLabels...)
		t.update(labels, count)
		t.getLogger().Debug("TCP flag metric", zap.String("flag", flag), zap.Strings("labels", labels), zap.Uint32("count", count))
	}
}

func (t *TCPMetrics) processLocalCtxFlow(flow *v1.Flow, flags []string) {
	labelValuesMap := t.sourceCtx().getLocalCtxValues(flow)
	if labelValuesMap == nil {
		return
	}

	combinedFlags := combineFlagsWithPrevious(flags, flow)

	// Ingress values
	if l := len(labelValuesMap[ingress]); l > 0 {
		for flag, count := range combinedFlags {
			labels := append([]string{flag}, labelValuesMap[ingress]...)
			t.update(labels, count)
			t.getLogger().Debug("TCP flag metric", zap.String("flag", flag), zap.Strings("labels", labels), zap.Uint32("count", count))
		}
	}

	if l := len(labelValuesMap[egress]); l > 0 {
		for flag, count := range combinedFlags {
			labels := append([]string{flag}, labelValuesMap[egress]...)
			t.update(labels, count)
			t.getLogger().Debug("TCP flag metric", zap.String("flag", flag), zap.Strings("labels", labels), zap.Uint32("count", count))
		}
	}
}

func (t *TCPMetrics) expire(labels []string) bool {
	var d bool
	if t.tcpFlagsMetrics != nil {
		d = t.tcpFlagsMetrics.DeleteLabelValues(labels...)
		if d {
			metricsinit.MetricsExpiredCounter.WithLabelValues(TCPFlagsCountName).Inc()
		}
	}
	return d
}

func (t *TCPMetrics) update(labels []string, count uint32) {
	t.tcpFlagsMetrics.WithLabelValues(labels...).Add(float64(count))
	t.updated(labels)
}

func (t *TCPMetrics) getFlagValues(flags *v1.TCPFlags) []string {
	f := make([]string, 0)
	if flags == nil {
		return f
	}

	if flags.GetFIN() {
		f = append(f, utils.FIN)
	}

	if flags.GetSYN() && flags.GetACK() {
		f = append(f, utils.SYNACK)
	} else {
		if flags.GetSYN() {
			f = append(f, utils.SYN)
		}
		if flags.GetACK() {
			f = append(f, utils.ACK)
		}
	}
	if flags.GetRST() {
		f = append(f, utils.RST)
	}

	if flags.GetPSH() {
		f = append(f, utils.PSH)
	}

	if flags.GetURG() {
		f = append(f, utils.URG)
	}

	if flags.GetECE() {
		f = append(f, utils.ECE)
	}

	if flags.GetCWR() {
		f = append(f, utils.CWR)
	}

	if flags.GetNS() {
		f = append(f, utils.NS)
	}

	return f
}

func (t *TCPMetrics) Clean() {
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(t.tcpFlagsMetrics))
	t.clean()
}
