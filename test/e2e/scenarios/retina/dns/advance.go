// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	"fmt"
	"log"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
	"github.com/pkg/errors"
)

var (
	dnsAdvRequestCountMetricName  = "networkobservability_adv_dns_request_count"
	dnsAdvResponseCountMetricName = "networkobservability_adv_dns_response_count"
)

type ValidateAdvanceDNSRequestMetrics struct {
	IP           string
	Namespace    string
	NumResponse  string
	PodName      string
	Query        string
	QueryType    string
	Response     string
	ReturnCode   string
	WorkloadKind string
	WorkloadName string
}

func (v *ValidateAdvanceDNSRequestMetrics) Run() error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%d/metrics", common.RetinaPort)
	// Get Pod IP address
	podIP, err := kubernetes.GetPodIP("", v.Namespace, v.PodName)
	if err != nil {
		return err
	}
	v.IP = podIP

	validateAdvanceDNSRequestMetrics := map[string]string{
		"ip":            v.IP,
		"namespace":     v.Namespace,
		"num_response":  v.NumResponse,
		"podname":       v.PodName,
		"query":         v.Query,
		"query_type":    v.QueryType,
		"response":      v.Response,
		"return_code":   v.ReturnCode,
		"workload_kind": v.WorkloadKind,
		"workload_name": v.WorkloadName,
	}

	err = prom.CheckMetric(metricsEndpoint, dnsAdvRequestCountMetricName, validateAdvanceDNSRequestMetrics)
	if err != nil {
		return errors.Wrapf(err, "failed to verify advance dns request metrics %s", dnsAdvRequestCountMetricName)
	}
	log.Printf("found metrics matching %+v\n", dnsAdvRequestCountMetricName)

	return nil
}

func (v *ValidateAdvanceDNSRequestMetrics) Prevalidate() error {
	return nil
}

func (v *ValidateAdvanceDNSRequestMetrics) Stop() error {
	return nil
}

type ValidateAdvanceDNSResponseMetrics struct {
	IP           string
	Namespace    string
	NumResponse  string
	PodName      string
	Query        string
	QueryType    string
	Response     string
	ReturnCode   string
	WorkloadKind string
	WorkloadName string
}

func (v *ValidateAdvanceDNSResponseMetrics) Run() error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%d/metrics", common.RetinaPort)
	// Get Pod IP address
	podIP, err := kubernetes.GetPodIP("", v.Namespace, v.PodName)
	if err != nil {
		return err
	}
	v.IP = podIP

	validateAdvanceDNSResponseMetrics := map[string]string{
		"ip":            v.IP,
		"namespace":     v.Namespace,
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
