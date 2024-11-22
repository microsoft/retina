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
	DroppedPacketsGaugeName              = "drop_count"
	DropBytesGaugeName                   = "drop_bytes"
	ForwardPacketsGaugeName              = "forward_count"
	ForwardBytesGaugeName                = "forward_bytes"
	TCPStateGaugeName                    = "tcp_state"
	TCPConnectionRemoteGaugeName         = "tcp_connection_remote"
	TCPConnectionStatsName               = "tcp_connection_stats"
	TCPFlagGauge                         = "tcp_flag_gauges"
	TCPRetransCount                      = "tcp_retransmission_count"
	IPConnectionStatsName                = "ip_connection_stats"
	UDPConnectionStatsName               = "udp_connection_stats"
	InterfaceStatsName                   = "interface_stats"
	DNSRequestCounterName                = "dns_request_count"
	DNSResponseCounterName               = "dns_response_count"
	NodeAPIServerLatencyName             = "node_apiserver_latency"
	NodeAPIServerTCPHandshakeLatencyName = "node_apiserver_handshake_latency"
	NoResponseFromAPIServerName          = "node_apiserver_no_response"
	InfinibandCounterStatsName           = "infiniband_counter_stats"
	InfinibandStatusParamsName           = "infiniband_status_params"
	ConntrackPacketsCounterName          = "packets_count_per_connection"
	ConntrackBytesCounterName            = "bytes_count_per_connection"
	ConntrackConnectionsCounterName      = "connections_count"

	// Common Gauges across os distributions
	NodeConnectivityStatusName         = "node_connectivity_status"
	NodeConnectivityLatencySecondsName = "node_connectivity_latency_seconds"
)

// IsAdvancedMetric is a helper function to determine if a name is an advanced metric
func IsAdvancedMetric(name string) bool {
	switch name {
	case
		DroppedPacketsGaugeName,
		DropBytesGaugeName,
		ForwardPacketsGaugeName,
		ForwardBytesGaugeName,
		NodeConnectivityStatusName,
		NodeConnectivityLatencySecondsName,
		TCPStateGaugeName,
		TCPConnectionRemoteGaugeName,
		TCPConnectionStatsName,
		TCPFlagGauge,
		TCPRetransCount,
		IPConnectionStatsName,
		UDPConnectionStatsName,
		DNSRequestCounterName,
		DNSResponseCounterName,
		NodeAPIServerLatencyName,
		NodeAPIServerTCPHandshakeLatencyName,
		NoResponseFromAPIServerName,
		ConntrackPacketsCounterName,
		ConntrackBytesCounterName,
		ConntrackConnectionsCounterName:
		return true
	default:
		return false
	}
}
