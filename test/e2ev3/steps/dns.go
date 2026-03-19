// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"context"
	"fmt"
	"log"

	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
)

// EmptyResponse is a sentinel value that gets converted to an empty string
// for metric label matching (workaround for the framework not supporting empty values directly).
const EmptyResponse = "emptyResponse"

var (
	dnsBasicRequestCountMetricName  = "networkobservability_dns_request_count"
	dnsBasicResponseCountMetricName = "networkobservability_dns_response_count"
	dnsAdvRequestCountMetricName    = "networkobservability_adv_dns_request_count"
	dnsAdvResponseCountMetricName   = "networkobservability_adv_dns_response_count"
)

// ValidateBasicDNSRequestStep checks that the basic DNS request count metric exists.
type ValidateBasicDNSRequestStep struct{}

func (v *ValidateBasicDNSRequestStep) Do(_ context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	validBasicDNSRequestMetricLabels := map[string]string{}

	err := prom.CheckMetric(metricsEndpoint, dnsBasicRequestCountMetricName, validBasicDNSRequestMetricLabels)
	if err != nil {
		return fmt.Errorf("failed to verify basic dns request metrics %s: %w", dnsBasicRequestCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsBasicRequestCountMetricName)
	return nil
}

// ValidateBasicDNSResponseStep checks that the basic DNS response count metric exists.
type ValidateBasicDNSResponseStep struct {
	NumResponse string
	Query       string
	QueryType   string
	ReturnCode  string
	Response    string
}

func (v *ValidateBasicDNSResponseStep) Do(_ context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	if v.Response == EmptyResponse {
		v.Response = ""
	}

	validBasicDNSResponseMetricLabels := map[string]string{}

	err := prom.CheckMetric(metricsEndpoint, dnsBasicResponseCountMetricName, validBasicDNSResponseMetricLabels)
	if err != nil {
		return fmt.Errorf("failed to verify basic dns response metrics %s: %w", dnsBasicResponseCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsBasicResponseCountMetricName)
	return nil
}

// ValidateAdvancedDNSRequestStep checks the advanced DNS request count metric
// with labels including pod IP, namespace, pod name, query info, and workload info.
type ValidateAdvancedDNSRequestStep struct {
	PodNamespace       string
	PodName            string
	Query              string
	QueryType          string
	WorkloadKind       string
	WorkloadName       string
	KubeConfigFilePath string
}

func (v *ValidateAdvancedDNSRequestStep) Do(_ context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	podIP, err := kubernetes.GetPodIP(v.KubeConfigFilePath, v.PodNamespace, v.PodName)
	if err != nil {
		return fmt.Errorf("failed to get pod IP address: %w", err)
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
		return fmt.Errorf("failed to verify advance dns request metrics %s: %w", dnsAdvRequestCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsAdvRequestCountMetricName)
	return nil
}

// ValidateAdvancedDNSResponseStep checks the advanced DNS response count metric
// with labels including pod IP, namespace, pod name, num_response, query info,
// response, return_code, and workload info.
type ValidateAdvancedDNSResponseStep struct {
	PodNamespace       string
	NumResponse        string
	PodName            string
	Query              string
	QueryType          string
	Response           string
	ReturnCode         string
	WorkloadKind       string
	WorkloadName       string
	KubeConfigFilePath string
}

func (v *ValidateAdvancedDNSResponseStep) Do(_ context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	podIP, err := kubernetes.GetPodIP(v.KubeConfigFilePath, v.PodNamespace, v.PodName)
	if err != nil {
		return fmt.Errorf("failed to get pod IP address: %w", err)
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
		return fmt.Errorf("failed to verify advance dns response metrics %s: %w", dnsAdvResponseCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsAdvResponseCountMetricName)
	return nil
}
