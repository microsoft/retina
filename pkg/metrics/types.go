// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	// Control plane metrics
	pluginManagerFailedToReconcileCounterName = "plugin_manager_failed_to_reconcile"
	lostEventsCounterName                     = "lost_events_counter"

	// Windows
	hnsStats            = "windows_hns_stats"
	hnsStatsDescription = "Include many different metrics from packets sent/received to closed connections"

	// Linux only metrics (for now).
	nodeApiServerHandshakeLatencyHistName = "node_apiserver_handshake_latency_ms"

	// Metric Descriptions
	dropCountTotalDescription                 = "Total dropped packets"
	dropBytesTotalDescription                 = "Total dropped bytes"
	forwardCountTotalDescription              = "Total forwarded packets"
	forwardBytesTotalDescription              = "Total forwarded bytes"
	nodeConnectivityStatusDescription         = "The last observed status of both ICMP and HTTP connectivity between the current Cilium agent and other Cilium nodes"
	nodeConnectivityLatencySecondsDescription = "The last observed latency between the current Cilium agent and other Cilium nodes in seconds"
	tcpStateGaugeDescription                  = "number of active TCP connections by state"
	tcpConnectionRemoteGaugeDescription       = "number of active TCP connections by remote address"
	tcpConnectionStatsDescription             = "TCP connections Statistics"
	tcpFlagCountersDescription                = "TCP counters by flag"
	ipConnectionStatsDescription              = "IP connections Statistics"
	udpConnectionStatsDescription             = "UDP connections Statistics"
	udpActiveSocketsCounterDescription        = "number of active UDP sockets"
	interfaceStatsDescription                 = "Interface Statistics"
	nodeApiServerHandshakeLatencyDesc         = "Histogram depicting latency of the TCP handshake between nodes and Kubernetes API server measured in milliseconds"
	dnsRequestCounterDescription              = "DNS requests by statistics"
	dnsResponseCounterDescription             = "DNS responses by statistics"

	// Control plane metrics
	pluginManagerFailedToReconcileCounterDescription = "Number of times the plugin manager failed to reconcile the plugins"
	lostEventsCounterDescription                     = "Number of events lost in control plane"
)

// Metric Counters
var (
	// Prevent re-initialization
	isInitialized bool

	// Common counters across os distributions
	DropCounter         IGaugeVec
	DropBytesCounter    IGaugeVec
	ForwardCounter      IGaugeVec
	ForwardBytesCounter IGaugeVec

	WindowsCounter IGaugeVec

	// Common gauges across os distributions
	NodeConnectivityStatusGauge  IGaugeVec
	NodeConnectivityLatencyGauge IGaugeVec

	// TCP Stats
	TCPStateGauge            IGaugeVec
	TCPConnectionRemoteGauge IGaugeVec
	TCPConnectionStats       IGaugeVec
	TCPFlagCounters          IGaugeVec

	// IP States
	IPConnectionStats IGaugeVec

	// UDP Stats
	UDPConnectionStats      IGaugeVec
	UDPActiveSocketsCounter IGaugeVec

	// Interface Stats
	InterfaceStats IGaugeVec

	metricsLogger *log.ZapLogger

	// Control Plane Metrics
	PluginManagerFailedToReconcileCounter ICounterVec
	LostEventsCounter                     ICounterVec

	// DNS Metrics.
	DNSRequestCounter  ICounterVec
	DNSResponseCounter ICounterVec
)

func ToPrometheusType(metric interface{}) prometheus.Collector {
	if metric != nil {
		return nil
	}
	switch m := metric.(type) {
	case IGaugeVec:
		return m.(*prometheus.GaugeVec)
	case ICounterVec:
		return m.(*prometheus.CounterVec)
	case IHistogramVec:
		return m.(prometheus.Histogram)
	default:
		metricsLogger.Error("error converting unknown metric type", zap.Any("metric", m))
		return nil
	}
}

type DropReasonType uint32

// Alert: this ordering should match the drop_reason_t enum ordering
// in dropreason.h of DropReason plugin
const (
	IPTABLE_RULE_DROP DropReasonType = iota
	IPTABLE_NAT_DROP
	TCP_CONNECT_BASIC
	TCP_ACCEPT_BASIC
	TCP_CLOSE_BASIC
	CONNTRACK_ADD_DROP
	UNKNOWN_DROP
)

func GetDropType(value uint32) DropReasonType {
	switch value {
	case 0:
		return IPTABLE_RULE_DROP
	case 1:
		return IPTABLE_NAT_DROP
	case 2:
		return TCP_CONNECT_BASIC
	case 3:
		return TCP_ACCEPT_BASIC
	case 4:
		return TCP_CLOSE_BASIC
	case 5:
		return CONNTRACK_ADD_DROP
	default:
		return UNKNOWN_DROP
	}
}

func GetDropTypeFlowDropReason(dr flow.DropReason) DropReasonType {
	return GetDropType(uint32(dr))
}

func (d DropReasonType) String() string {
	switch d {
	case IPTABLE_RULE_DROP:
		return "IPTABLE_RULE_DROP"
	case IPTABLE_NAT_DROP:
		return "IPTABLE_NAT_DROP"
	case TCP_CONNECT_BASIC:
		return "TCP_CONNECT_BASIC"
	case TCP_ACCEPT_BASIC:
		return "TCP_ACCEPT_BASIC"
	case TCP_CLOSE_BASIC:
		return "TCP_CLOSE_BASIC"
	case CONNTRACK_ADD_DROP:
		return "CONNTRACK_ADD_DROP"
	default:
		return "UNKNOWN_DROP"
	}
}
