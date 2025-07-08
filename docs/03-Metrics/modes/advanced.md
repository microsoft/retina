# Advanced Metrics

There are two Advanced modes (see [Metric Modes](./modes.md)) which include all [Basic Metrics](./basic.md) plus extra metrics providing Pod-Level context.

The two Advanced modes are *remote context* and *local context*. The difference lies in the [Context Labels](#context-labels).
Additionally, *local context* supports [Annotations](../annotations.md).

## Prefix

All metrics have the prefix `networkobservability_`.

## Universal Labels

Node and Cluster metadata are included with the labels:

- `cluster`
- `instance` (Node name)

## Context Labels

There are Pod-Level context labels for metrics prepended with `adv_`.

To customize context labels, see [MetricsConfiguration CRD](../../05-Concepts/CRDs/MetricsConfiguration.md).

### Remote Context

For Advanced mode with *remote context*, default context labels are the following:

- `source_ip`
- `source_namespace`
- `source_pod`
- `source_workload`
- `source_zone`
- `destination_ip`
- `destination_namespace`
- `destination_pod`
- `destination_workload`
- `destination_zone`

### Local Context

For Advanced mode with *local context*, default context labels are the following for *outgoing* traffic:

- `source_ip`
- `source_namespace`
- `source_pod`
- `source_workload`

For *incoming* traffic:

- `destination_ip`
- `destination_namespace`
- `destination_pod`
- `destination_workload`

## List of Metrics

### Plugin: `packetforward` (Linux)

[Same metrics](./basic.md#plugin-packetforward-linux) as Basic mode.

### Plugin: `dropreason` (Linux)

Metrics enabled when `dropreason` plugin is enabled (see [Metrics Configuration](../configuration.md)).

| Metric Name             | Description                                    | Extra Labels          |
| ----------------------- | ---------------------------------------------- | --------------------- |
| `drop_count`            | *Basic*: dropped packet count                  | `direction`, `reason` |
| `drop_bytes`            | *Basic*: dropped byte count                    | `direction`, `reason` |
| `adv_drop_count`        | ***Advanced/Pod-Level***: dropped packet count | `direction`, `reason`, context labels |
| `adv_drop_bytes`        | ***Advanced/Pod-Level***: dropped byte count   | `direction`, `reason`, context labels |

#### Label Values

See [Context Labels](#context-labels).

Possible values for `direction`:

- `ingress` (incoming traffic)
- `egress` (outgoing traffic)

Possible values for `reason`:

- `IPTABLE_RULE_DROP`
- `IPTABLE_NAT_DROP`
- `TCP_CONNECT_BASIC`
- `TCP_ACCEPT_BASIC`
- `TCP_CLOSE_BASIC`
- `CONNTRACK_ADD_DROP`
- `UNKNOWN_DROP`

### Plugin: `linuxutil` (Linux)

[Same metrics](./basic.md#plugin-linuxutil-linux) as Basic mode.

### Plugin: `dns` (Linux)

Metrics enabled when `dns` plugin is enabled (see [Metrics Configuration](../configuration.md)).

| Metric Name                  | Description                                                                                    | Extra Labels                                                                     |
| ---------------------------- | ---------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `dns_request_count`              | *Basic*: number of DNS requests by query                                                   | `query_type`, `query`                                                            |
| `dns_response_count`             | *Basic*: number of DNS responses by query, error code, and response value                  | `query_type`, `query`, `return_code`, `response`, `num_response`                 |
| `adv_dns_request_count`          | ***Advanced/Pod-Level***: number of DNS requests by query                                  | `query_type`, `query`, context labels                                            |
| `adv_dns_response_count`         | ***Advanced/Pod-Level***: number of DNS responses by query, error code, and response value | `query_type`, `query`, `return_code`, `response`, `num_response`, context labels |

### Plugin: `hnsstats` (Windows)

[Same metrics](./basic.md#plugin-hnsstats-windows) as Basic mode.

### Plugin: `packetparser` (Linux)

Metrics enabled when `packetparser` plugin is enabled (see [Metrics Configuration](../configuration.md)).

| Metric Name                                | Description                                                                   | Extra Labels                |
| ------------------------------------------ | ----------------------------------------------------------------------------- | --------------------------- |
| `adv_forward_count`                        | ***Advanced/Pod-Level***: forwarded packet count                              | `direction`, context labels |
| `adv_forward_bytes`                        | ***Advanced/Pod-Level***: forwarded byte count                                | `direction`, context labels |
| `adv_tcpflags_count`                       | ***Advanced/Pod-Level***: TCP packet count by flag                            | `flag`, context labels      |
| `adv_node_apiserver_latency`               | ***Advanced***: API Server round trip time for SYN-ACK (histogram)            | `le` (histogram bucket)     |
| `adv_node_apiserver_no_response`           | ***Advanced***: number of packets that did not get a response from API server |                             |
| `adv_node_apiserver_tcp_handshake_latency` | ***Advanced***: API Server latency in establishing connection (histogram)     | `le` (histogram bucket)     |

Note: API Server metrics help identify degradation of Node-to-API-server connection.
The metrics were born out of a real-life incident, where Node-to-API-server latency was the root cause.

#### Label Values

See [Context Labels](#context-labels).

Possible values for `direction`:

- `ingress` (incoming traffic)
- `egress` (outgoing traffic)

Possible values for `flag`:

- `FIN`
- `SYN`
- `RST`
- `PSH`
- `ACK`
- `URG`
- `ECE`
- `CWR`
- `NS`

Possible values for `le` (for API server metrics). Units are in *milliseconds*. `le` stands for "less than or equal". See [Prometheus histogram documentation](https://prometheus.io/docs/concepts/metric_types/#histogram) for more info.

- `0`
- `0.5`
- `1` through `4.5` in increments of 0.5
- `inf`

### Plugin: `tcpretrans` (Linux)

Metrics enabled when `tcpretrans` plugin is enabled (see [Metrics Configuration](../configuration.md)).

| Metric Name            | Description                                              | Extra Labels   |
| ---------------------- | -------------------------------------------------------- | -------------- |
| `adv_tcpretrans_count` | ***Advanced/Pod-Level***: TCP retransmitted packet count | context labels |

#### Label Values

See [Context Labels](#context-labels).
