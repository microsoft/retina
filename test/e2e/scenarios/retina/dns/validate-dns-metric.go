// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	"fmt"
	"log"

	"github.com/microsoft/retina/test/e2e/common"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

const (
	numResponse = "num_response"
	query       = "query"
	queryType   = "query_type"
	returnCode  = "return_code"
	response    = "response"
)

var (
	dnsRequestCountMetricName   = "networkobservability_dns_request_count"
	dnsResponseCountMetricName  = "networkobservability_dns_response_count"
	validDNSRequestMetricLabels = map[string]string{
		numResponse: "0",
		query:       "kubernetes.default.svc.cluster.local.",
		queryType:   "AAAA",
		returnCode:  "",
		response:    "",
	}
	validDNSResponseMetricLabels = map[string]string{
		numResponse: "0",
		query:       "kubernetes.default.svc.cluster.local.",
		queryType:   "AAAA",
		returnCode:  "NoError",
		response:    "",
	}
)

type ValidateDNSRequestMetrics struct{}

func (v *ValidateDNSRequestMetrics) Run() error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%d/metrics", common.RetinaPort)

	err := prom.CheckMetric(metricsEndpoint, dnsRequestCountMetricName, validDNSRequestMetricLabels)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dnsRequestCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsRequestCountMetricName)

	return nil
}

func (v *ValidateDNSRequestMetrics) Prevalidate() error {
	return nil
}

func (v *ValidateDNSRequestMetrics) Stop() error {
	return nil
}

type ValidateDNSResponseMetrics struct{}

func (v *ValidateDNSResponseMetrics) Run() error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%d/metrics", common.RetinaPort)

	err := prom.CheckMetric(metricsEndpoint, dnsResponseCountMetricName, validDNSResponseMetricLabels)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dnsResponseCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsResponseCountMetricName)

	return nil
}

func (v *ValidateDNSResponseMetrics) Prevalidate() error {
	return nil
}

func (v *ValidateDNSResponseMetrics) Stop() error {
	return nil
}
