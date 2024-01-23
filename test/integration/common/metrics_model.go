// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build integration
// +build integration

package integration

import (
	prom_exporter "github.com/microsoft/retina/pkg/exporter"
)

type MetricType int

const (
	// dropreason and forward metrics
	Count MetricType = iota
	Bytes
	// dns metrics
	Request
	Response
	// linuxutil metrics
	IPMetricType
	TCPMetricType
	UDPMetricType
)

// Prefix for all metrics
var RetinaPrefix = prom_exporter.RetinaNamespace + "_"

// Drop related metrics
var (
	DropReasonCountMetricName         = RetinaPrefix + "drop_count"
	DropReasonBytesMetricName         = RetinaPrefix + "drop_bytes"
	AdvancedDropReasonCountMetricName = RetinaPrefix + "adv_drop_count"
	AdvancedDropReasonBytesMetricName = RetinaPrefix + "adv_drop_bytes"
)

// Forward related metrics
var (
	ForwardCountMetricName         = RetinaPrefix + "forward_count"
	ForwardBytesMetricName         = RetinaPrefix + "forward_bytes"
	AdvancedForwardCountMetricName = RetinaPrefix + "adv_forward_count"
	AdvancedForwardBytesMetricName = RetinaPrefix + "adv_forward_bytes"
)

// DNS related metrics
var (
	DNSRequestMetricName  = RetinaPrefix + "adv_dns_request_count"
	DNSResponseMetricName = RetinaPrefix + "adv_dns_response_count"
)

// Linuxutil related metrics
var (
	LinuxUtilInterfaceStatsMetricName      = RetinaPrefix + "interface_stats"
	LinuxUtilIpConnectionStatsMetricName   = RetinaPrefix + "ip_connection_stats"
	LinuxUtilTcpConnectionRemoteMetricName = RetinaPrefix + "tcp_connection_remote"
	LinuxUtilTcpConnectionStatsMetricName  = RetinaPrefix + "tcp_connection_stats"
	LinuxUtilTcpStateMetricName            = RetinaPrefix + "tcp_state"
	LinuxUtilUdpConnectionStatsMetricName  = RetinaPrefix + "udp_connection_stats"
)

// Other advanced metrics
var (
	AdvancedTcpFlagsCountMetricName = RetinaPrefix + "adv_tcpflags_count"
)

type BaseObj struct {
	Ip           string
	Namespace    string
	PodName      string
	WorkloadKind string
	WorkloadName string
}

type ModelBasicDropReasonMetrics struct {
	Direction string
	Reason    string
}

// NewModelBasicDropReasonMetrics creates a ModelBasicDropReasonMetrics
func NewModelBasicDropReasonMetrics(direction, reason string) *ModelBasicDropReasonMetrics {
	return &ModelBasicDropReasonMetrics{
		Direction: direction,
		Reason:    reason,
	}
}

// Given a ModelBasicDropReasonMetrics, convert it to list of labels
func (m *ModelBasicDropReasonMetrics) WithLabels(metricType MetricType) MetricWithLabels {
	var metricName string
	switch metricType {
	case Count:
		metricName = DropReasonCountMetricName
	case Bytes:
		metricName = DropReasonBytesMetricName
	}
	return MetricWithLabels{
		Metric: metricName,
		Labels: []string{
			"direction", m.Direction,
			"reason", m.Reason,
		},
	}
}

type ModelLocalCtxDropReasonMetrics struct {
	ModelBasicDropReasonMetrics
	BaseObj
}

func NewModelLocalCtxDropReasonMetrics(direction, ip, namespace, podname, reason, workloadKind, workloadName string) *ModelLocalCtxDropReasonMetrics {
	return &ModelLocalCtxDropReasonMetrics{
		ModelBasicDropReasonMetrics: *NewModelBasicDropReasonMetrics(direction, reason),
		BaseObj: BaseObj{
			Ip:           ip,
			Namespace:    namespace,
			PodName:      podname,
			WorkloadKind: workloadKind,
			WorkloadName: workloadName,
		},
	}
}

// Given a ModelLocalCtxDropReasonMetrics, convert it to list of labels
func (m *ModelLocalCtxDropReasonMetrics) WithLabels(metricType MetricType) MetricWithLabels {
	labels := m.ModelBasicDropReasonMetrics.WithLabels(metricType).Labels
	labels = append(labels, []string{
		"ip", m.BaseObj.Ip,
		"namespace", m.BaseObj.Namespace,
		"podname", m.BaseObj.PodName,
		"workloadKind", m.BaseObj.WorkloadKind,
		"workloadName", m.BaseObj.WorkloadName,
	}...)

	var metricName string
	switch metricType {
	case Count:
		metricName = AdvancedDropReasonCountMetricName
	case Bytes:
		metricName = AdvancedDropReasonBytesMetricName
	}

	return MetricWithLabels{
		Metric: metricName,
		Labels: labels,
	}
}

type ModelTcpFlagsMetrics struct {
	Flag string
	BaseObj
}

func NewModelTcpFlagsMetrics(flag, ip, namespace, podname, workloadKind, workloadName string) *ModelTcpFlagsMetrics {
	return &ModelTcpFlagsMetrics{
		Flag: flag,
		BaseObj: BaseObj{
			Ip:           ip,
			Namespace:    namespace,
			PodName:      podname,
			WorkloadKind: workloadKind,
			WorkloadName: workloadName,
		},
	}
}

// Given a ModelTcpFlagsMetrics, convert it to list of labels
func (m *ModelTcpFlagsMetrics) WithLabels() MetricWithLabels {
	return MetricWithLabels{
		Metric: AdvancedTcpFlagsCountMetricName,
		Labels: []string{
			"flag", m.Flag,
			"ip", m.BaseObj.Ip,
			"namespace", m.BaseObj.Namespace,
			"podname", m.BaseObj.PodName,
			"workloadKind", m.BaseObj.WorkloadKind,
			"workloadName", m.BaseObj.WorkloadName,
		},
	}
}

type ModelBasicForwardMetrics struct {
	Direction string
}

func NewModelBasicForwardMetrics(direction string) *ModelBasicForwardMetrics {
	return &ModelBasicForwardMetrics{
		Direction: direction,
	}
}

// Given a ModelBasicForwardCountMetrics, convert it to list of labels
func (m *ModelBasicForwardMetrics) WithLabels(metricType MetricType) MetricWithLabels {
	var metricName string
	switch metricType {
	case Count:
		metricName = ForwardCountMetricName
	case Bytes:
		metricName = ForwardBytesMetricName
	}

	return MetricWithLabels{
		Metric: metricName,
		Labels: []string{
			"direction", m.Direction,
		},
	}
}

type ModelLocalCtxForwardMetrics struct {
	ModelBasicForwardMetrics
	BaseObj
}

func NewModelLocalCtxForwardMetrics(direction, ip, namespace, podname, workloadKind, workloadName string) *ModelLocalCtxForwardMetrics {
	return &ModelLocalCtxForwardMetrics{
		ModelBasicForwardMetrics: *NewModelBasicForwardMetrics(direction),
		BaseObj: BaseObj{
			Ip:           ip,
			Namespace:    namespace,
			PodName:      podname,
			WorkloadKind: workloadKind,
			WorkloadName: workloadName,
		},
	}
}

// Given a ModelLocalCtxForwardMetrics, convert it to list of labels
func (m *ModelLocalCtxForwardMetrics) WithLabels(metricType MetricType) MetricWithLabels {
	labels := m.ModelBasicForwardMetrics.WithLabels(metricType).Labels
	labels = append(labels, []string{
		"ip", m.BaseObj.Ip,
		"namespace", m.BaseObj.Namespace,
		"podname", m.BaseObj.PodName,
		"workloadKind", m.BaseObj.WorkloadKind,
		"workloadName", m.BaseObj.WorkloadName,
	}...)

	var metricName string
	switch metricType {
	case Count:
		metricName = AdvancedForwardCountMetricName
	case Bytes:
		metricName = AdvancedForwardBytesMetricName
	}

	return MetricWithLabels{
		Metric: metricName,
		Labels: labels,
	}
}

type ModelDnsCountMetrics struct {
	NumResponse string
	Query       string
	QueryType   string
	Response    string
	ReturnCode  string
	BaseObj
}

func NewModelDnsCountMetrics(numResponse, query, queryType, response, returnCode, ip, namespace, podname, workloadKind, workloadName string) *ModelDnsCountMetrics {
	return &ModelDnsCountMetrics{
		NumResponse: numResponse,
		Query:       query,
		QueryType:   queryType,
		Response:    response,
		ReturnCode:  returnCode,
		BaseObj: BaseObj{
			Ip:           ip,
			Namespace:    namespace,
			PodName:      podname,
			WorkloadKind: workloadKind,
			WorkloadName: workloadName,
		},
	}
}

func (m *ModelDnsCountMetrics) WithLabels(metricType MetricType) MetricWithLabels {
	var metricName string
	switch metricType {
	case Request:
		metricName = DNSRequestMetricName
	case Response:
		metricName = DNSResponseMetricName
	}

	return MetricWithLabels{
		Metric: metricName,
		Labels: []string{
			"num_response", m.NumResponse,
			"query", m.Query,
			"query_type", m.QueryType,
			"response", m.Response,
			"return_code", m.ReturnCode,
			"ip", m.BaseObj.Ip,
			"namespace", m.BaseObj.Namespace,
			"podname", m.BaseObj.PodName,
			"workloadKind", m.BaseObj.WorkloadKind,
			"workloadName", m.BaseObj.WorkloadName,
		},
	}
}

type MetricWithLabels struct {
	Metric string
	Labels []string
}
