package dns

import (
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

var (
	dnsRequestCountMetricName  = "networkobservability_dns_request_count"
	dnsResponseCountMetricName = "networkobservability_dns_response_count"
)

const (
	numResponseKey = "num_response"
	queryKey       = "query"
	queryTypeKey   = "query_type"
	returnCodeKey  = "return_code"
	responseKey    = "response"
)

type DnsMetric struct {
	DnsRetinaPort string
	NumResponse   string
	Query         string
	QueryType     string
	ReturnCode    string
	Response      string
}

type ValidateDnsRequest struct {
	DnsMetric
}

type ValidateDnsResponse struct {
	DnsMetric
}

func (v *ValidateDnsRequest) Run() error {
	return v.runMetric(dnsRequestCountMetricName)
}

func (v *ValidateDnsResponse) Run() error {
	return v.runMetric(dnsResponseCountMetricName)
}

func (v *DnsMetric) runMetric(metricName string) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.DnsRetinaPort)

	metric := map[string]string{
		numResponseKey: v.NumResponse, queryKey: v.Query, queryTypeKey: v.QueryType, returnCodeKey: v.ReturnCode, responseKey: v.Response,
	}

	err := prom.CheckMetric(promAddress, metricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", metricName, err)
	}

	log.Printf("found metrics matching %+v\n", metric)
	return nil
}

func (v *ValidateDnsRequest) Prevalidate() error {
	return nil
}

func (v *ValidateDnsRequest) Stop() error {
	return nil
}

func (v *ValidateDnsResponse) Prevalidate() error {
	return nil
}

func (v *ValidateDnsResponse) Stop() error {
	return nil
}
