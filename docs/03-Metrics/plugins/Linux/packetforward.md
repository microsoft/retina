# `packetforward`

Counts number of packets/bytes passing through the `eth0` interface of a Node, along with the direction of the packets.

## Capabilities

The `packetforward` plugin requires the `CAP_BPF` and `CAP_NET_RAW` capabilities.
- `CAP_NET_RAW` is used to open raw sockets on the `eth0` network interface - `OpenRawSocket()` method at `packetforward_linux.go:159`

## Architecture

`packetforward` uses an eBPF socket filter program on the host's primary interface to capture packets and generate basic metrics from the captured data.

### Code locations

- Plugin and eBPF code: *pkg/plugin/packetforward/*

## Metrics

See metrics for [Basic Mode](../../modes/basic.md#plugin-packetforward-linux) (Advanced modes have identical metrics).

:::note

`adv_forward_count` and `adv_forward_bytes` metrics are NOT associated with `packetforward` plugin despite similarities in name.
These metrics are associated with [`packetparser`](./packetparser.md).

:::
