// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	"fmt"
	"log"

	"github.com/microsoft/retina/test/e2e/framework/constants"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
	"github.com/pkg/errors"
)

var (
	dnsAdvRequestCountMetricName  = "networkobservability_adv_dns_request_count"
	dnsAdvResponseCountMetricName = "networkobservability_adv_dns_response_count"
)

type ValidateAdvancedDNSRequestMetrics struct {
	PodNamespace string
	PodName      string
	Query        string
	QueryType    string
	WorkloadKind string
	WorkloadName string

	KubeConfigFilePath string
}

func (v *ValidateAdvancedDNSRequestMetrics) Run() error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", constants.RetinaMetricsPort)
	// Get Pod IP address
	podIP, err := kubernetes.GetPodIP(v.KubeConfigFilePath, v.PodNamespace, v.PodName)
	if err != nil {
		return errors.Wrapf(err, "failed to get pod IP address")
	}

	validateAdvancedDNSRequestMetrics := map[string]string{
		"ip":            podIP,
		"namespace":     v.PodNamespace,
		"podname":       v.PodName,
		"query":         v.Query,
		"query_type":    v.QueryType,
		"workload_kind": v.WorkloadKind,
		"workload_name": v.WorkloadName,
	}

	err = prom.CheckMetric(metricsEndpoint, dnsAdvRequestCountMetricName, validateAdvancedDNSRequestMetrics)
	if err != nil {
		return errors.Wrapf(err, "failed to verify advance dns request metrics %s", dnsAdvRequestCountMetricName)
	}
	log.Printf("found metrics matching %+v\n", dnsAdvRequestCountMetricName)

	return nil
}

func (v *ValidateAdvancedDNSRequestMetrics) Prevalidate() error {
	return nil
}

func (v *ValidateAdvancedDNSRequestMetrics) Stop() error {
	return nil
}

type ValidateAdvanceDNSResponseMetrics struct {
	PodNamespace string
	NumResponse  string
	PodName      string
	Query        string
	QueryType    string
	Response     string
	ReturnCode   string
	WorkloadKind string
	WorkloadName string

	KubeConfigFilePath string
}

func (v *ValidateAdvanceDNSResponseMetrics) Run() error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", constants.RetinaMetricsPort)
	// Get Pod IP address
	podIP, err := kubernetes.GetPodIP(v.KubeConfigFilePath, v.PodNamespace, v.PodName)
	if err != nil {
		return errors.Wrapf(err, "failed to get pod IP address")
	}

	if v.Response == EmptyResponse {
		v.Response = ""
	}

	validateAdvanceDNSResponseMetrics := map[string]string{
		"ip":            podIP,
		"namespace":     v.PodNamespace,
		"num_response":  v.NumResponse,
		"podname":       v.PodName,
		"query":         v.Query,
		"query_type":    v.QueryType,
		"response":      v.Response,
		"return_code":   v.ReturnCode,
		"workload_kind": v.WorkloadKind,
		"workload_name": v.WorkloadName,
	}

	err = prom.CheckMetric(metricsEndpoint, dnsAdvResponseCountMetricName, validateAdvanceDNSResponseMetrics)
	if err != nil {
		return errors.Wrapf(err, "failed to verify advance dns response metrics %s", dnsAdvRequestCountMetricName)
	}
	log.Printf("found metrics matching %+v\n", dnsAdvResponseCountMetricName)

	return nil
}

func (v *ValidateAdvanceDNSResponseMetrics) Prevalidate() error {
	return nil
}

func (v *ValidateAdvanceDNSResponseMetrics) Stop() error {
	return nil
}
