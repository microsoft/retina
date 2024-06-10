// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

/* Creating this UTILs package to avoid  operator from importing filtermanager package
 *
 * This package will contain all the utility functions that are used by the operator
 * needed to make metrics work
 */

const (
	// Common Counters across os distributions (should these be asynch or synch)
	// make sure IsMetric is updated if you add a new metric here
	DropCountTotalName                   = "drop_count"
	DropBytesTotalName                   = "drop_bytes"
	ForwardCountTotalName                = "forward_count"
	ForwardBytesTotalName                = "forward_bytes"
	TcpStateGaugeName                    = "tcp_state"
	TcpConnectionRemoteGaugeName         = "tcp_connection_remote"
	TcpConnectionStatsName               = "tcp_connection_stats"
	TcpFlagCounters                      = "tcp_flag_counters"
	TcpRetransCount                      = "tcp_retransmission_count"
	IpConnectionStatsName                = "ip_connection_stats"
	UdpConnectionStatsName               = "udp_connection_stats"
	UdpActiveSocketsCounterName          = "udp_active_sockets"
	InterfaceStatsName                   = "interface_stats"
	DNSRequestCounterName                = "dns_request_count"
	DNSResponseCounterName               = "dns_response_count"
	NodeApiServerLatencyName             = "node_apiserver_latency"
	NodeApiServerTcpHandshakeLatencyName = "node_apiserver_handshake_latency"
	NoResponseFromApiServerName          = "node_apiserver_no_response"
	InfinibandCounterStatsName           = "infiniband_counter_stats"
	InfinibandStatusParamsName           = "infiniband_status_params"

	// Common Gauges across os distributions
	NodeConnectivityStatusName         = "node_connectivity_status"
	NodeConnectivityLatencySecondsName = "node_connectivity_latency_seconds"
)

// IsAdvancedMetric is a helper function to determine if a name is an advanced metric
func IsAdvancedMetric(name string) bool {
	switch name {
	case
		DropCountTotalName,
		DropBytesTotalName,
		ForwardCountTotalName,
		ForwardBytesTotalName,
		NodeConnectivityStatusName,
		NodeConnectivityLatencySecondsName,
		TcpStateGaugeName,
		TcpConnectionRemoteGaugeName,
		TcpConnectionStatsName,
		TcpFlagCounters,
		TcpRetransCount,
		IpConnectionStatsName,
		UdpConnectionStatsName,
		UdpActiveSocketsCounterName,
		DNSRequestCounterName,
		DNSResponseCounterName,
		NodeApiServerLatencyName,
		NodeApiServerTcpHandshakeLatencyName,
		NoResponseFromApiServerName:
		return true
	default:
		return false
	}
}
