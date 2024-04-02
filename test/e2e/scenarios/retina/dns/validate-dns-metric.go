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

type Metric struct {
	DNSRetinaPort string
	NumResponse   string
	Query         string
	QueryType     string
	ReturnCode    string
	Response      string
}

type ValidateDNSRequest struct {
	Metric
}

type ValidateDNSResponse struct {
	Metric
}

func (v *ValidateDNSRequest) Run() error {
	return v.runMetric(dnsRequestCountMetricName)
}

func (v *ValidateDNSResponse) Run() error {
	return v.runMetric(dnsResponseCountMetricName)
}

func (v *Metric) runMetric(metricName string) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.DNSRetinaPort)

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

func (v *ValidateDNSRequest) Prevalidate() error {
	return nil
}

func (v *ValidateDNSRequest) Stop() error {
	return nil
}

func (v *ValidateDNSResponse) Prevalidate() error {
	return nil
}

func (v *ValidateDNSResponse) Stop() error {
	return nil
}
