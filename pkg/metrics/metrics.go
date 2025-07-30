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
		utils.DroppedPacketsGaugeName,
		dropPacketsGaugeDescription,
		utils.Reason,
		utils.Direction)
	DropBytesGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.DropBytesGaugeName,
		dropBytesGaugeDescription,
		utils.Reason,
		utils.Direction)
	ForwardPacketsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ForwardPacketsGaugeName,
		forwardPacketsGaugeDescription,
		utils.Direction)
	ForwardBytesGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ForwardBytesGaugeName,
		forwardBytesGaugeDescription,
		utils.Direction)
	HNSStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		hnsStats,
		hnsStatsDescription,
		utils.Direction,
	)
	NodeConnectivityStatusGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.NodeConnectivityStatusName,
		nodeConnectivityStatusGaugeDescription,
		utils.SourceNodeName,
		utils.TargetNodeName)
	NodeConnectivityLatencyGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.NodeConnectivityLatencySecondsName,
		nodeConnectivityLatencySecondsGaugeDescription,
		utils.SourceNodeName,
		utils.TargetNodeName)

	TCPStateGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.TCPStateGaugeName,
		tcpStateGaugeDescription,
		utils.State,
	)
	TCPConnectionRemoteGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.TCPConnectionRemoteGaugeName,
		tcpConnectionRemoteGaugeDescription,
		utils.Address,
	)
	TCPConnectionStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.TCPConnectionStatsName,
		tcpConnectionStatsGaugeDescription,
		utils.StatName,
	)

	TCPFlagGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.TCPFlagGauge,
		tcpFlagGaugeDescription,
		utils.Direction,
		utils.Flag,
	)

	// IP States
	IPConnectionStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.IPConnectionStatsName,
		ipConnectionStatsGaugeDescription,
		utils.StatName,
	)

	// UDP Stats
	UDPConnectionStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.UDPConnectionStatsName,
		udpConnectionStatsGaugeDescription,
		utils.StatName,
	)

	// Interface Stats
	InterfaceStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.InterfaceStatsName,
		interfaceStatsGaugeDescription,
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
	)
	DNSResponseCounter = exporter.CreatePrometheusCounterVecForMetric(
		exporter.DefaultRegistry,
		utils.DNSResponseCounterName,
		dnsResponseCounterDescription,
	)

	// InfiniBand Metrics
	InfinibandStatsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.InfinibandCounterStatsName,
		infinibandStatsGaugeDescription,
		utils.StatName,
		utils.Device,
		utils.Port,
	)

	InfinibandStatusParamsGauge = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.InfinibandStatusParamsName,
		infinibandStatusParamsGaugeDescription,
		utils.StatName,
		utils.InterfaceName,
	)

	ConntrackPacketsTx = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ConntrackPacketsTxGaugeName,
		ConntrackPacketTxDescription,
	)

	ConntrackPacketsRx = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ConntrackPacketsRxGaugeName,
		ConntrackPacketRxDescription,
	)

	ConntrackBytesTx = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ConntrackBytesTxGaugeName,
		ConntrackBytesTxDescription,
	)

	ConntrackBytesRx = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ConntrackBytesRxGaugeName,
		ConntrackBytesRxDescription,
	)

	ConntrackTotalConnections = exporter.CreatePrometheusGaugeVecForMetric(
		exporter.DefaultRegistry,
		utils.ConntrackTotalConnectionsName,
		ConntrackTotalConnectionsDescription,
	)

	ParsedPacketsCounter = exporter.CreatePrometheusCounterVecForControlPlaneMetric(
		exporter.DefaultRegistry,
		parsedPacketsCounterName,
		parsedPacketsCounterDescription,
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
