// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"github.com/microsoft/retina/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	// Control plane metrics
	pluginManagerFailedToReconcileCounterName = "plugin_manager_failed_to_reconcile"
	lostEventsCounterName                     = "lost_events_counter"
	parsedPacketsCounterName                  = "parsed_packets_counter"

	// Windows
	hnsStats            = "windows_hns_stats"
	hnsStatsDescription = "Include many different metrics from packets sent/received to closed connections"

	// Linux only metrics (for now).
	nodeApiServerHandshakeLatencyHistName = "node_apiserver_handshake_latency_ms"

	// Metric Descriptions
	dropPacketsGaugeDescription                    = "Total dropped packets"
	dropBytesGaugeDescription                      = "Total dropped bytes"
	forwardPacketsGaugeDescription                 = "Total forwarded packets"
	forwardBytesGaugeDescription                   = "Total forwarded bytes"
	nodeConnectivityStatusGaugeDescription         = "The last observed status of both ICMP and HTTP connectivity between the current Cilium agent and other Cilium nodes"
	nodeConnectivityLatencySecondsGaugeDescription = "The last observed latency between the current Cilium agent and other Cilium nodes in seconds"
	tcpStateGaugeDescription                       = "Number of active TCP connections by state"
	tcpConnectionRemoteGaugeDescription            = "Number of active TCP connections by remote address"
	tcpConnectionStatsGaugeDescription             = "TCP connections statistics"
	tcpFlagGaugeDescription                        = "TCP gauges by flag"
	ipConnectionStatsGaugeDescription              = "IP connections statistics"
	udpConnectionStatsGaugeDescription             = "UDP connections statistics"
	interfaceStatsGaugeDescription                 = "Interface statistics"
	nodeAPIServerHandshakeLatencyDesc              = "Histogram depicting latency of the TCP handshake between nodes and Kubernetes API server measured in milliseconds"
	dnsRequestCounterDescription                   = "DNS requests by statistics"
	dnsResponseCounterDescription                  = "DNS responses by statistics"
	infinibandStatsGaugeDescription                = "InfiniBand statistics gauge"
	infinibandStatusParamsGaugeDescription         = "InfiniBand Status Parameters gauge"

	// Control plane metrics
	pluginManagerFailedToReconcileCounterDescription = "Number of times the plugin manager failed to reconcile the plugins"
	lostEventsCounterDescription                     = "Number of events lost in control plane"
	parsedPacketsCounterDescription                  = "Number of packets parsed by the packetparser plugin"

	// Conntrack metrics
	ConntrackPacketTxDescription         = "Number of tx packets"
	ConntrackPacketRxDescription         = "Number of rx packets"
	ConntrackBytesTxDescription          = "Number of tx bytes"
	ConntrackBytesRxDescription          = "Number of rx bytes"
	ConntrackTotalConnectionsDescription = "Total number of connections"
)

// Metric Counters
var (
	// Prevent re-initialization
	isInitialized bool

	// Common gauges across os distributions
	DropPacketsGauge    GaugeVec
	DropBytesGauge      GaugeVec
	ForwardPacketsGauge GaugeVec
	ForwardBytesGauge   GaugeVec

	// Windows
	HNSStatsGauge GaugeVec

	// Common gauges across os distributions
	NodeConnectivityStatusGauge  GaugeVec
	NodeConnectivityLatencyGauge GaugeVec

	// TCP Stats
	TCPStateGauge            GaugeVec
	TCPConnectionRemoteGauge GaugeVec
	TCPConnectionStatsGauge  GaugeVec
	TCPFlagGauge             GaugeVec

	// IP States
	IPConnectionStatsGauge GaugeVec

	// UDP Stats
	UDPConnectionStatsGauge GaugeVec

	// Interface Stats
	InterfaceStatsGauge GaugeVec

	metricsLogger *log.ZapLogger

	// Control Plane Metrics
	PluginManagerFailedToReconcileCounter CounterVec
	LostEventsCounter                     CounterVec
	ParsedPacketsCounter                  CounterVec

	// DNS Metrics.
	DNSRequestCounter  CounterVec
	DNSResponseCounter CounterVec

	InfinibandStatsGauge        GaugeVec
	InfinibandStatusParamsGauge GaugeVec

	// Conntrack
	ConntrackPacketsTx        GaugeVec
	ConntrackPacketsRx        GaugeVec
	ConntrackBytesTx          GaugeVec
	ConntrackBytesRx          GaugeVec
	ConntrackTotalConnections GaugeVec
)

func ToPrometheusType(metric interface{}) prometheus.Collector {
	if metric != nil {
		return nil
	}
	switch m := metric.(type) {
	case GaugeVec:
		return m.(*prometheus.GaugeVec)
	case CounterVec:
		return m.(*prometheus.CounterVec)
	default:
		metricsLogger.Error("error converting unknown metric type", zap.Any("metric", m))
		return nil
	}
}

type DropReasonType uint32
