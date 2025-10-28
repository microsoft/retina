// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
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
	baseMetricInterface
	dnsMetrics metricsinit.CounterVec
	metricName string
}

func NewDNSMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext, ttl time.Duration) *DNSMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), "dns") {
		return nil
	}

	fl = fl.Named("dns-metricsmodule")
	fl.Info("Creating DNS count metrics", zap.Any("options", ctxOptions))
	d := &DNSMetrics{}
	d.baseMetricInterface = newBaseMetricsObject(ctxOptions, fl, isLocalContext, d.expire, ttl)
	return d
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
			d.getRequestLabels()...,
		)
	case utils.DNSResponseCounterName:
		d.dnsMetrics = exporter.CreatePrometheusCounterVecForMetric(
			exporter.AdvancedRegistry,
			DNSResponseCountName,
			DNSResponseCountDesc,
			d.getResponseLabels()...,
		)
	}
}

func (d *DNSMetrics) getRequestLabels() []string {
	labels := utils.DNSRequestLabels
	if d.sourceCtx() != nil {
		labels = append(labels, d.sourceCtx().getLabels()...)
		d.getLogger().Info("src labels", zap.Any("labels", labels))
	}

	if d.destinationCtx() != nil {
		labels = append(labels, d.destinationCtx().getLabels()...)
		d.getLogger().Info("dst labels", zap.Any("labels", labels))
	}

	return labels
}

func (d *DNSMetrics) getResponseLabels() []string {
	labels := utils.DNSResponseLabels
	if d.sourceCtx() != nil {
		labels = append(labels, d.sourceCtx().getLabels()...)
		d.getLogger().Info("src labels", zap.Any("labels", labels))
	}

	if d.destinationCtx() != nil {
		labels = append(labels, d.destinationCtx().getLabels()...)
		d.getLogger().Info("dst labels", zap.Any("labels", labels))
	}

	return labels
}

func (d *DNSMetrics) requestValues(flow *v1.Flow) []string {
	flowDNS, dnsType, _ := utils.GetDNS(flow)
	if flowDNS == nil {
		return nil
	}
	if dnsType == utils.DNSType_UNKNOWN ||
		(d.metricName == utils.DNSRequestCounterName && dnsType != utils.DNSType_QUERY) ||
		(d.metricName == utils.DNSResponseCounterName && dnsType != utils.DNSType_RESPONSE) {
		return nil
	}

	labels := []string{
		strings.Join(flowDNS.GetQtypes(), ","),
		flowDNS.GetQuery(),
	}
	return labels
}

func (d *DNSMetrics) responseValues(flow *v1.Flow) []string {
	flowDNS, dnsType, numResponses := utils.GetDNS(flow)
	if flowDNS == nil {
		return nil
	}
	if dnsType == utils.DNSType_UNKNOWN ||
		(d.metricName == utils.DNSRequestCounterName && dnsType != utils.DNSType_QUERY) ||
		(d.metricName == utils.DNSResponseCounterName && dnsType != utils.DNSType_RESPONSE) {
		return nil
	}

	labels := []string{
		utils.DNSRcodeToString(flow),
		strings.Join(flowDNS.GetQtypes(), ","),
		flowDNS.GetQuery(),
		strings.Join(flowDNS.GetIps(), ","),
		strconv.FormatUint(uint64(numResponses), 10),
	}
	return labels
}

func (d *DNSMetrics) getLabelsForProcessFlow(flow *v1.Flow) ([]string, error) {
	var labels []string
	// Get the DNS query type
	_, dnsType, _ := utils.GetDNS(flow)
	switch dnsType {
	case utils.DNSType_QUERY:
		labels = d.requestValues(flow)
	case utils.DNSType_RESPONSE:
		labels = d.responseValues(flow)
	case utils.DNSType_UNKNOWN:
	default:
		return labels, errors.Errorf("invalid DNS type %d", int32(dnsType))
	}
	return labels, nil
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

	labels, err := d.getLabelsForProcessFlow(flow)
	if err != nil {
		d.getLogger().Error("Failed to get labels for process flow", zap.Error(err))
		return
	}

	if len(labels) == 0 {
		return
	}

	if d.sourceCtx() != nil {
		srcLabels := d.sourceCtx().getValues(flow)
		if len(srcLabels) > 0 {
			labels = append(labels, srcLabels...)
		}
	}

	if d.destinationCtx() != nil {
		dstLabels := d.destinationCtx().getValues(flow)
		if len(dstLabels) > 0 {
			labels = append(labels, dstLabels...)
		}
	}

	d.update(labels)
	d.getLogger().Debug("Update dns metric in remote ctx", zap.Any("metric", d.dnsMetrics), zap.Any("labels", labels))
}

func (d *DNSMetrics) processLocalCtxFlow(flow *v1.Flow) {
	labelValuesMap := d.sourceCtx().getLocalCtxValues(flow)
	if labelValuesMap == nil {
		return
	}

	labels, err := d.getLabelsForProcessFlow(flow)
	if err != nil {
		d.getLogger().Error("Failed to get labels for process flow", zap.Error(err))
		return
	}

	if len(labels) == 0 {
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
	d.update(labels)
	d.getLogger().Debug("Update dns metric in local ctx", zap.Any("metric", d.dnsMetrics), zap.Any("labels", labels))
}

func (d *DNSMetrics) expire(labels []string) bool {
	var del bool
	if d.dnsMetrics != nil {
		del = d.dnsMetrics.DeleteLabelValues(labels...)
		if del {
			metricsinit.MetricsExpiredCounter.WithLabelValues(d.metricName).Inc()
		}
	}
	return del
}

func (d *DNSMetrics) update(labels []string) {
	d.dnsMetrics.WithLabelValues(labels...).Inc()
	d.updated(labels)
}

func (d *DNSMetrics) Clean() {
	exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(d.dnsMetrics))
	d.clean()
}
