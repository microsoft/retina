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
	TCPFlagsCountName = "adv_tcpflags_count"

	// Metric descriptions
	TCPFlagsCountDesc = "Total number of packets by TCP flag"
)

type TCPMetrics struct {
	baseMetricObject
	tcpFlagsMetrics  metricsinit.GaugeVec
	metricName       string
	enableStandalone bool
}

func NewTCPMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext, enableStandalone bool) *TCPMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "flag") {
		return nil
	}

	fl = fl.Named("tcpflags-metricsmodule")
	fl.Info("Creating TCP Flags count metrics", zap.Any("options", ctxOptions))
	return &TCPMetrics{
		baseMetricObject: newBaseMetricsObject(ctxOptions, fl, isLocalContext),
		enableStandalone: enableStandalone,
	}
}

func (t *TCPMetrics) Init(metricName string) {
	// only 1 metric. No need to check metric name which is already validated.
	t.tcpFlagsMetrics = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.AdvancedRegistry,
		TCPFlagsCountName,
		TCPFlagsCountDesc,
		t.getLabels()...,
	)
	t.metricName = metricName
}

func (t *TCPMetrics) getLabels() []string {
	labels := []string{
		utils.Flag,
	}

	if t.enableStandalone {
		labels = append(labels, utils.Direction)
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

	if t.enableStandalone {
		t.processStandaloneFlow(flow)
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
	if t.srcCtx != nil {
		srcLabels = t.srcCtx.getValues(flow)
	}

	if t.dstCtx != nil {
		dstLabels = t.dstCtx.getValues(flow)
	}

	for flag, count := range combineFlagsWithPrevious(flags, flow) {
		labels := append([]string{flag}, srcLabels...)
		labels = append(labels, dstLabels...)
		t.tcpFlagsMetrics.WithLabelValues(labels...).Add(float64(count))
		t.l.Debug("TCP flag metric", zap.String("flag", flag), zap.Strings("labels", labels), zap.Uint32("count", count))
	}
}

func (t *TCPMetrics) processLocalCtxFlow(flow *v1.Flow, flags []string) {
	labelValuesMap := t.srcCtx.getLocalCtxValues(flow)
	if labelValuesMap == nil {
		return
	}

	combinedFlags := combineFlagsWithPrevious(flags, flow)

	// Ingress values
	if l := len(labelValuesMap[ingress]); l > 0 {
		for flag, count := range combinedFlags {
			labels := append([]string{flag}, labelValuesMap[ingress]...)
			t.tcpFlagsMetrics.WithLabelValues(labels...).Add(float64(count))
			t.l.Debug("TCP flag metric", zap.String("flag", flag), zap.Strings("labels", labels), zap.Uint32("count", count))
		}
	}

	if l := len(labelValuesMap[egress]); l > 0 {
		for flag, count := range combinedFlags {
			labels := append([]string{flag}, labelValuesMap[egress]...)
			t.tcpFlagsMetrics.WithLabelValues(labels...).Add(float64(count))
			t.l.Debug("TCP flag metric", zap.String("flag", flag), zap.Strings("labels", labels), zap.Uint32("count", count))
		}
	}
}

func (t *TCPMetrics) processStandaloneFlow(fl *v1.Flow) {
	if GetHNSMetadata(fl).GetVfpPortStatsData() == nil {
		return
	}

	// ingress values
	ingressLbls := []string{
		ingress,
		fl.GetIP().GetSource(),
		fl.GetSource().GetNamespace(),
		fl.GetSource().GetPodName(),
		"",
		"",
	}

	// egress values
	egressLbls := []string{
		egress,
		fl.GetIP().GetSource(),
		fl.GetSource().GetNamespace(),
		fl.GetSource().GetPodName(),
		"",
		"",
	}

	tcpInStats := GetHNSMetadata(fl).GetVfpPortStatsData().GetIn().GetTcpCounters().GetPacketCounters()
	t.tcpFlagsMetrics.WithLabelValues(append([]string{utils.SYN}, ingressLbls...)...).Set(float64(tcpInStats.GetSynPacketCount()))
	t.tcpFlagsMetrics.WithLabelValues(append([]string{utils.SYNACK}, ingressLbls...)...).Set(float64(tcpInStats.GetSynAckPacketCount()))
	t.tcpFlagsMetrics.WithLabelValues(append([]string{utils.FIN}, ingressLbls...)...).Set(float64(tcpInStats.GetFinPacketCount()))
	t.tcpFlagsMetrics.WithLabelValues(append([]string{utils.RST}, ingressLbls...)...).Set(float64(tcpInStats.GetRstPacketCount()))

	tcpOutStats := GetHNSMetadata(fl).GetVfpPortStatsData().GetOut().GetTcpCounters().GetPacketCounters()
	t.tcpFlagsMetrics.WithLabelValues(append([]string{utils.SYN}, egressLbls...)...).Set(float64(tcpOutStats.GetSynPacketCount()))
	t.tcpFlagsMetrics.WithLabelValues(append([]string{utils.SYNACK}, egressLbls...)...).Set(float64(tcpOutStats.GetSynAckPacketCount()))
	t.tcpFlagsMetrics.WithLabelValues(append([]string{utils.FIN}, egressLbls...)...).Set(float64(tcpOutStats.GetFinPacketCount()))
	t.tcpFlagsMetrics.WithLabelValues(append([]string{utils.RST}, egressLbls...)...).Set(float64(tcpOutStats.GetRstPacketCount()))
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
	t.l.Info("Cleaning metric", zap.String("name", t.metricName))
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(t.tcpFlagsMetrics))
}
