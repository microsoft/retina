# `packetparser` (Linux)

Measures TCP packets passing through `eth0`, providing the ability to calculate TCP-handshake latencies, etc.

## Metrics

See metrics for [Advanced Mode](../modes/advanced.md#plugin-packetparser-linux). For module information, see [below](#modules).

## Architecture

The plugin utilizes eBPF to gather data.
The plugin does not generate Basic metrics.
In Advanced mode (see [Metric Modes](../modes/modes.md)), the plugin turns an eBPF result into an enriched `Flow` (adding Pod information based on IP), then sends the `Flow` to an external channel so that *several modules* can create Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/tcpretrans/*
- Modules for extra Advanced metrics: see section below.

### Modules

#### Module: forward

Code path: *pkg/module/metrics/forward.go*

Metrics produced:

- `adv_forward_count`
- `adv_forward_bytes`

#### Module: tcpflags

Code path: *pkg/module/metrics/tcpflags.go*

Metrics produced:

- `adv_forward_count`
- `adv_forward_bytes`

#### Module: latency (API Server)

Code path: *pkg/module/metrics/latency.go*

Metrics produced:

- `adv_node_apiserver_latency`
- `adv_node_apiserver_no_response`
- `adv_node_apiserver_tcp_handshake_latency`
