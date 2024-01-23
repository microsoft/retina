# Basic Metrics

These metrics are provided in all three modes (see [Metric Modes](./modes.md)).

## Prefix

All metrics have the prefix `networkobservability_`.

## Universal Labels

All metrics include node and Cluster metadata with the labels:
- `cluster`
- `instance` (Node name)

## List of Metrics

### Plugin: `packetforward` (Linux)

Metrics enabled when `packetforward` plugin is enabled (see [Metrics Configuration](./configuration.md)).

|        Name             | Description              | Extra Labels  |
| ----------------------- | -----------------------  | ------------- |
| `forward_count`         | forwarded packet count   | `direction`   |
| `forward_bytes`         | forwarded byte count     | `direction`   |

#### Label Values

Possible values for `direction`:
- `ingress` (incoming traffic)
- `egress` (outgoing traffic)

### Plugin: `dropreason` (Linux)

Metrics enabled when `dropreason` plugin is enabled (see [Metrics Configuration](./configuration.md)).

| Metric Name             | Description              | Extra Labels          |
| ----------------------- | ------------------------ | --------------------- |
| `drop_count`            | dropped packet count     | `direction`, `reason` |
| `drop_bytes`            | dropped byte count       | `direction`, `reason` |

#### Label Values

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

Metrics enabled when `linuxutil` plugin is enabled (see [Metrics Configuration](./configuration.md)).

| Metric Name             | Description                                                                     | Extra Labels                       |
| ----------------------- | ------------------------------------------------------------------------------- | ---------------------------------- |
| `tcp_state`             | TCP currently active socket count by TCP state (from `netstats` utility)        | `state`                            |
| `tcp_connection_remote` | TCP currently active socket count by remote IP/port  (from `netstats` utility)  | `address` (IP), `port`             |
| `tcp_connection_stats`  | TCP connection statistics  (from `netstats` utility)                            | `statistic_name`                   |
| `ip_connection_stats`   | IP connection statistics  (from `netstats` utility)                             | `statistic_name`                   |
| `udp_connection_stats`  | UDP connection statistics (from `netstats` utility)                             | `statistic_name`                   |
| `interface_stats`       | interface statistics (from `ethtool` utility)                                   | `interface_name`, `statistic_name` |

#### Label Values

Possible values for TCP `state`:
- `UNKNOWN`
- `ESTABLISHED`
- `SYN_SENT`
- `SYN_RECV`
- `FIN_WAIT1`
- `FIN_WAIT2`
- `TIME_WAIT`
- `""` (represents CLOSE)
- `CLOSE_WAIT`
- `LAST_ACK`
- `LISTEN`
- `CLOSING`

Possible values for `statistic_name` (for metric `tcp_connection_stats`):
- `TCPTimeouts`
- `TCPTSReorder`
- `ResetCount`
- and many others (full list [here](./plugins/linuxutil.md#label-values-for-tcp_connection_stats))

Possible values for `statistic_name` (for metric `ip_connection_stats`):
- `InNoECTPkts`
- `InNoRoutes`
- `InOctets`
- `OutOctets`

Possible values for `statistic_name` (for metric `udp_connection_stats`):
- `ACTIVE` (currently active socket count)

Possible values for `statistic_name` (for metric `interface_stats`):
- `tx_packets`
- `rx_packets`
- `rx0_cache_full` (or `rx1_`, etc.)
- `tx0_nop` (or `tx1_`, etc.)
- `rx_dropped`
- `tx_dropped`
- `rx_comp_full`
- `tx_send_full`
- and many others (as seen by running `ethtool -S <interface_name>` on the Node)

### Plugin: `dns` (Linux)

Metrics enabled when `dns` plugin is enabled (see [Metrics Configuration](./configuration.md)).

| Metric Name                  | Description                                                      | Extra Labels                                                     |
| ---------------------------- | ---------------------------------------------------------------- | ---------------------------------------------------------------- |
| `dns_request_count`          | number of DNS requests by query                                  | `query_type`, `query`                                            |
| `dns_response_count`         | number of DNS responses by query, error code, and response value | `query_type`, `query`, `return_code`, `response`, `num_response` |

### Plugin: `hnsstats` (Windows)

Metrics enabled when `hnsstats` plugin is enabled (see [Metrics Configuration](./configuration.md)).

|        Name             | Description                                   | Extra Labels          |
| ----------------------- | ----------------------------------------------| --------------------- |
| `forward_count`         | forwarded packet count (from HNS)             | `direction`           |
| `forward_bytes`         | forwarded byte count (from HNS)               | `direction`           |
| `drop_count`            | dropped packet count (from HNS or VFP)        | `direction`, `reason` |
| `drop_bytes`            | dropped byte count (from HNS or VFP)          | `direction`, `reason` |
| `tcp_connection_stats`  | several TCP connection statistics (from VFP)  | `statistic_name`      |
| `tcp_flag_counters`     | TCP packet count by flag (from VFP)           | `direction`, `flag`   |

#### Label Values

Possible values for `direction`:
- `ingress` (incoming traffic)
- `egress` (outgoing traffic)

Possible values for `reason`:
- `endpoint` (dropped by HNS endpoint)
- `aclrule` (dropped by ACL firewall rule in VFP)

Possible values for `statistic_name` (for metric `tcp_connection_stats`):
- `ResetCount`
- `ClosedFin`
- `ResetSyn`
- `TcpHalfOpenTimeouts`
- `Verified`
- `TimedOutCount`
- `TimeWaitExpiredCount`

Possible values for TCP `flag`:
- `SYN`
- `SYNACK`
- `FIN`
- `RST`
