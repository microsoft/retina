# Hubble Metrics

When Retina is deployed with Hubble control plane, the metrics include Node-level and Pod-level. Metrics are stored in Prometheus format, and can be viewed in Grafana.

## Metrics Endpoints

The Hubble control plane exposes metrics on two separate ports:

* **Port 10093**: Node-level metrics (`networkobservability_*` prefix)
* **Port 9965**: Hubble pod-level metrics (`hubble_*` prefix)

## Metrics

* Node-Level Metrics: These metrics provide insights into traffic volume, dropped packets, number of connections, etc. by node. Available on port 10093.
* Hubble Metrics (DNS and Pod-Level Metrics): These metrics include source and destination pod information allowing to pinpoint network-related issues at a granular level. Metrics cover DNS queries/responses, L4/L7 packet flows, dropped packets, and TCP flags. Available on port 9965.

### Node-Level Metrics

The following metrics are aggregated per node and available on port **10093**. All metrics include labels:

* `cluster`
* `instance` (Node name)

Retina provides metrics for both Linux and Windows operating systems.
The table below outlines the different metrics generated.

| Metric Name                                    | Description | Extra Labels | Linux | Windows |
|------------------------------------------------|-------------|--------------|-------|---------|
| **networkobservability_forward_count**         | Total forwarded packet count | `direction` | ✅ | ✅ |
| **networkobservability_forward_bytes**         | Total forwarded byte count | `direction` | ✅ | ✅ |
| **networkobservability_drop_count**            | Total dropped packet count | `direction`, `reason` | ✅ | ✅ |
| **networkobservability_drop_bytes**            | Total dropped byte count | `direction`, `reason` | ✅ | ❌ |
| **networkobservability_tcp_state**             | TCP currently active socket count by TCP state. | `state` | ✅ | ❌ |
| **networkobservability_tcp_connection_remote** | TCP currently active socket count by remote address. | `address` (IP:port) | ✅ | ❌ |
| **networkobservability_tcp_connection_stats**  | TCP connection statistics. (ex: Delayed ACKs, TCPKeepAlive, TCPSackFailures) | `statistic_name` | ✅ | ✅ |
| **networkobservability_tcp_flag_gauges**       | TCP packets count by flag. | `direction`, `flag` | ❌ | ✅ |
| **networkobservability_ip_connection_stats**   | IP connection statistics. | `statistic_name` | ✅ | ❌ |
| **networkobservability_udp_connection_stats**  | UDP connection statistics. Includes active socket count with `statistic_name="ACTIVE"`. | `statistic_name` | ✅ | ❌ |
| **networkobservability_interface_stats**       | Interface statistics. | `interface_name`, `statistic_name` | ✅ | ❌ |
| **networkobservability_dns_request_count**     | Total DNS request count | | ✅ | ❌ |
| **networkobservability_dns_response_count**    | Total DNS response count | | ✅ | ❌ |
| **networkobservability_windows_hns_stats**     | Windows HNS statistics (packets sent/received) | `direction` | ❌ | ✅ |
| **networkobservability_node_connectivity_status** | Deprecated; registered but not populated | `source_node_name`, `target_node_name` | ❌ | ❌ |
| **networkobservability_node_connectivity_latency_seconds** | Deprecated; registered but not populated | `source_node_name`, `target_node_name` | ❌ | ❌ |

> **Note:** Linux `interface_stats` may expose no samples because it includes
> only non-zero `ethtool -S` counters containing `err` or `drop`.
> The deprecated node-connectivity collectors remain registered for
> compatibility but expose no metric series.

### Pod-Level Metrics (Hubble Metrics)

The following metrics are aggregated per pod (node information is preserved) and available on port **9965**. All metrics include labels:

* `cluster`
* `instance` (Node name)
* `source`
* `destination`

For *outgoing traffic*, there will be a `source` label with source pod namespace/name.
For *incoming traffic*, there will be a `destination` label with destination pod namespace/name.

| Metric Name                      | Description                  | Extra Labels          | Linux | Windows |
|----------------------------------|------------------------------|-----------------------|-------|---------|
| **hubble_dns_queries_total**     | Total DNS requests by query  | `source` or `destination`, `query`, `qtypes` (query type), `ips_returned`, `rcode` | ✅ | ❌ |
| **hubble_dns_responses_total**   | Total DNS responses by query/response | `source` or `destination`, `query`, `qtypes` (query type), `rcode` (return code), `ips_returned` (number of IPs) | ✅ | ❌ |
| **hubble_drop_total**            | Total dropped packet count | `source` or `destination`, `protocol`, `reason` | ✅ | ❌ |
| **hubble_flows_processed_total** | Total network flows processed (L4/L7 traffic) | `source` or `destination`, `protocol`, `verdict`, `type`, `subtype` | ✅ | ❌ |
| **hubble_tcp_flags_total**       | Total TCP packets count by flag. | `source` or `destination`, `flag` | ✅ | ❌ |
| **hubble_lost_events_total**     | Total number of lost Hubble events | | ✅ | ❌ |
