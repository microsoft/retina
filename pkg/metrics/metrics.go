// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// Initiates and creates the common metrics
func InitializeMetrics() {
	metricsLogger = log.Logger().Named("metrics")

	if isInitialized {
		metricsLogger.Warn("Metrics already initialized. Exiting.")
		return
	}
	DropPacketsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.DropCountTotalName,
		dropCountTotalDescription,
		utils.Reason,
		utils.Direction)
	DropBytesGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.DropBytesTotalName,
		dropBytesTotalDescription,
		utils.Reason,
		utils.Direction)
	ForwardPacketsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ForwardCountTotalName,
		forwardCountTotalDescription,
		utils.Direction)
	ForwardBytesGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ForwardBytesTotalName,
		forwardBytesTotalDescription,
		utils.Direction)
	WindowsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		hnsStats,
		hnsStatsDescription,
		utils.Direction,
	)
	NodeConnectivityStatusGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.NodeConnectivityStatusName,
		nodeConnectivityStatusDescription,
		utils.SourceNodeName,
		utils.TargetNodeName)
	NodeConnectivityLatencyGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.NodeConnectivityLatencySecondsName,
		nodeConnectivityLatencySecondsDescription,
		utils.SourceNodeName,
		utils.TargetNodeName)

	TCPStateGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.TcpStateGaugeName,
		tcpStateGaugeDescription,
		utils.State,
	)
	TCPConnectionRemoteGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.TcpConnectionRemoteGaugeName,
		tcpConnectionRemoteGaugeDescription,
		utils.Address,
		utils.Port,
	)
	TCPConnectionStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.TcpConnectionStatsName,
		tcpConnectionStatsDescription,
		utils.StatName,
	)

	// IP States
	IPConnectionStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.IpConnectionStatsName,
		ipConnectionStatsDescription,
		utils.StatName,
	)

	// UDP Stats
	UDPConnectionStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.UdpConnectionStatsName,
		udpConnectionStatsDescription,
		utils.StatName,
	)

	// Interface Stats
	InterfaceStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.InterfaceStatsName,
		interfaceStatsDescription,
		utils.InterfaceName,
		utils.StatName,
	)

	// Control Plane Metrics
	PluginManagerFailedToReconcileCounter = exporter.CreatePrometheusCounterVecForControlPlaneMetric(
		exporter.DefaultRegistry,
		pluginManagerFailedToReconcileCounterName,
		pluginManagerFailedToReconcileCounterDescription,
		utils.Reason,
	)

	// Lost Events defines the number of packets lost from reading eBPF maps
	LostEventsCounter = exporter.CreatePrometheusCounterVecForControlPlaneMetric(
		exporter.DefaultRegistry,
		lostEventsCounterName,
		lostEventsCounterDescription,
		utils.Type,
		utils.Reason,
	)

	// DNS Metrics.
	DNSRequestCounter = exporter.CreatePrometheusCounterVecForMetric(
		exporter.DefaultRegistry,
		utils.DNSRequestCounterName,
		dnsRequestCounterDescription,
		utils.DNSRequestLabels...,
	)
	DNSResponseCounter = exporter.CreatePrometheusCounterVecForMetric(
		exporter.DefaultRegistry,
		utils.DNSResponseCounterName,
		dnsResponseCounterDescription,
		utils.DNSResponseLabels...,
	)

	// InfiniBand Metrics
	InfinibandStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.InfinibandCounterStatsName,
		infinibandCounterStatsDescription,
		utils.StatName,
		utils.Device,
		utils.Port,
	)

	InfinibandStatusParamsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.InfinibandStatusParamsName,
		infinibandStatusParamsDescription,
		utils.StatName,
		utils.InterfaceName,
	)

	isInitialized = true
	metricsLogger.Info("Metrics initialized")
}

// GetCounterValue returns the current value
// stored for the counter
func GetCounterValue(m prometheus.Counter) float64 {
	var pm dto.Metric
	err := m.Write(&pm)
	if err == nil {
		return *pm.Counter.Value
	}
	return 0
}
