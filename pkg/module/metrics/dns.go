// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"fmt"
	"strings"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	// Metric descriptions
	DNSRequestCountDesc  = "Total number of DNS query packets"
	DNSResponseCountDesc = "Total number of DNS response packets"
)

var (
	DNSRequestCountName  = fmt.Sprintf("adv_%s", utils.DNSRequestCounterName)
	DNSResponseCountName = fmt.Sprintf("adv_%s", utils.DNSResponseCounterName)
)

type DNSMetrics struct {
	baseMetricObject
	dnsMetrics metricsinit.ICounterVec
	metricName string
}

func NewDNSMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext) *DNSMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "dns") {
		return nil
	}

	fl = fl.Named("dns-metricsmodule")
	fl.Info("Creating DNS count metrics", zap.Any("options", ctxOptions))
	return &DNSMetrics{
		baseMetricObject: newBaseMetricsObject(ctxOptions, fl, isLocalContext),
	}
}

func (d *DNSMetrics) Init(metricName string) {
	// TODO: Remove metricName from Init(). This makes the implementation of metrics
	// convoluted and difficult to understand.
	d.metricName = metricName
	switch metricName {
	case utils.DNSRequestCounterName:
		d.dnsMetrics = exporter.CreatePrometheusCounterVecForMetric(
			exporter.AdvancedRegistry,
			DNSRequestCountName,
			DNSRequestCountDesc,
			d.getLabels()...,
		)
	case utils.DNSResponseCounterName:
		d.dnsMetrics = exporter.CreatePrometheusCounterVecForMetric(
			exporter.AdvancedRegistry,
			DNSResponseCountName,
			DNSResponseCountDesc,
			d.getLabels()...,
		)
	}
}

func (d *DNSMetrics) getLabels() []string {
	labels := utils.DNSLabels
	if d.srcCtx != nil {
		labels = append(labels, d.srcCtx.getLabels()...)
		d.l.Info("src labels", zap.Any("labels", labels))
	}

	if d.dstCtx != nil {
		labels = append(labels, d.dstCtx.getLabels()...)
		d.l.Info("dst labels", zap.Any("labels", labels))
	}

	return labels
}

func (d *DNSMetrics) values(flow *flow.Flow) []string {
	flowDns, dnsType, numResponses := utils.GetDns(flow)
	if flowDns == nil {
		return nil
	}
	if dnsType == utils.DNSType_UNKNOWN ||
		(d.metricName == utils.DNSRequestCounterName && dnsType != utils.DNSType_QUERY) ||
		(d.metricName == utils.DNSResponseCounterName && dnsType != utils.DNSType_RESPONSE) {
		return nil
	}

	// Ref: DNSLabels {"return_code", "query_type", "query", "response", "num_response"}
	// "Response" label for DNS maybe empty. This is to avoid
	// https://github.com/inspektor-gadget/inspektor-gadget/issues/2008 .
	// Also ref: https://github.com/inspektor-gadget/inspektor-gadget/blob/main/docs/gadgets/trace/dns.md#limitations .
	labels := []string{
		utils.DnsRcodeToString(flow),
		strings.Join(flowDns.Qtypes, ","), flowDns.Query, strings.Join(flowDns.Ips, ","), fmt.Sprintf("%d", numResponses),
	}
	return labels
}

func (d *DNSMetrics) ProcessFlow(flow *v1.Flow) {
	if flow == nil {
		return
	}

	if flow.Verdict != utils.Verdict_DNS {
		return
	}

	if d.isLocalContext() {
		// when localcontext is enabled, we do not need the context options for both src and dst
		// metrics aggregation will be on a single pod basis and not the src/dst pod combination basis.
		d.processLocalCtxFlow(flow)
		return
	}

	labels := d.values(flow)
	if labels == nil {
		return
	}
	if d.srcCtx != nil {
		srcLabels := d.srcCtx.getValues(flow)
		if len(srcLabels) > 0 {
			labels = append(labels, srcLabels...)
		}
	}

	if d.dstCtx != nil {
		dstLabels := d.dstCtx.getValues(flow)
		if len(dstLabels) > 0 {
			labels = append(labels, dstLabels...)
		}
	}

	d.dnsMetrics.WithLabelValues(labels...).Inc()
	d.l.Debug("Update dns metric in remote ctx", zap.Any("metric", d.dnsMetrics), zap.Any("labels", labels))
}

func (d *DNSMetrics) processLocalCtxFlow(flow *v1.Flow) {
	labelValuesMap := d.srcCtx.getLocalCtxValues(flow)
	if labelValuesMap == nil {
		return
	}

	labels := d.values(flow)
	if labels == nil {
		return
	}

	if len(labelValuesMap[ingress]) > 0 && len(labelValuesMap[egress]) > 0 {
		// Check flow direction.
		if flow.TrafficDirection == v1.TrafficDirection_INGRESS {
			// For ingress flows, we add destination labels.
			labels = append(labels, labelValuesMap[ingress]...)
		} else {
			// For egress flows, we add source labels.
			labels = append(labels, labelValuesMap[egress]...)
		}
	} else if len(labelValuesMap[ingress]) > 0 {
		labels = append(labels, labelValuesMap[ingress]...)
	} else if len(labelValuesMap[egress]) > 0 {
		labels = append(labels, labelValuesMap[egress]...)
	} else {
		return
	}
	d.dnsMetrics.WithLabelValues(labels...).Inc()
	d.l.Debug("Update dns metric in local ctx", zap.Any("metric", d.dnsMetrics), zap.Any("labels", labels))
}

func (d *DNSMetrics) Clean() {
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(d.dnsMetrics))
}
