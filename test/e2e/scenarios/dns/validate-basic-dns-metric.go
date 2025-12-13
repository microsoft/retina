// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	"fmt"
	"log"

	"github.com/microsoft/retina/test/e2e/framework/constants"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
	"github.com/pkg/errors"
)

var (
	dnsBasicRequestCountMetricName  = "networkobservability_dns_request_count"
	dnsBasicResponseCountMetricName = "networkobservability_dns_response_count"
)

type validateBasicDNSRequestMetrics struct {
	Query     string
	QueryType string
}

func (v *validateBasicDNSRequestMetrics) Run() error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", constants.RetinaMetricsPort)

	validBasicDNSRequestMetricLabels := map[string]string{}

	err := prom.CheckMetric(metricsEndpoint, dnsBasicRequestCountMetricName, validBasicDNSRequestMetricLabels)
	if err != nil {
		return errors.Wrapf(err, "failed to verify basic dns request metrics %s", dnsBasicRequestCountMetricName)
	}
	log.Printf("found metrics matching %+v\n", dnsBasicRequestCountMetricName)

	return nil
}

func (v *validateBasicDNSRequestMetrics) Prevalidate() error {
	return nil
}

func (v *validateBasicDNSRequestMetrics) Stop() error {
	return nil
}

type validateBasicDNSResponseMetrics struct {
	NumResponse string
	Query       string
	QueryType   string
	ReturnCode  string
	Response    string
}

func (v *validateBasicDNSResponseMetrics) Run() error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", constants.RetinaMetricsPort)

	if v.Response == EmptyResponse {
		v.Response = ""
	}

	validBasicDNSResponseMetricLabels := map[string]string{}

	err := prom.CheckMetric(metricsEndpoint, dnsBasicResponseCountMetricName, validBasicDNSResponseMetricLabels)
	if err != nil {
		return errors.Wrapf(err, "failed to verify basic dns response metrics %s", dnsBasicResponseCountMetricName)
	}
	log.Printf("found metrics matching %+v\n", dnsBasicResponseCountMetricName)

	return nil
}

func (v *validateBasicDNSResponseMetrics) Prevalidate() error {
	return nil
}

func (v *validateBasicDNSResponseMetrics) Stop() error {
	return nil
}
