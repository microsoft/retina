# `packetparser` (Linux)

Captures TCP and UDP packets traveling to and from pods and nodes.

## Architecture

`packetparser` attached a [`qdisc` (Queuing Discipline)](https://www.man7.org/linux/man-pages/man8/tc.8.html) of type `clsact` to each pod's virtual interface (`veth`) and the host's default interface (`device`). This setup enabled the attachment of eBPF filter programs for both ingress and egress directions, allowing `packetparser` to capture individual packets traveling to and from the interfaces.

`packetparser` does not produce Basic metrics. In Advanced mode (refer to [Metric Modes](../modes/modes.md)), the plugin transforms an eBPF result into an enriched `Flow` by adding Pod information based on IP. It then sends the `Flow` to an external channel, enabling *several modules* to generate Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/packetparser/*
- Modules for extra Advanced metrics: see section below.

## Metrics

See metrics for [Advanced Mode](../modes/advanced.md#plugin-packetparser-linux). For module information, see [below](#modules).

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
