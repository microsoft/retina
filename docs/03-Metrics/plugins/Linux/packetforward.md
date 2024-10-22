# `packetforward`

Counts number of packets/bytes passing through the `eth0` interface of a Node, along with the direction of the packets.

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
