# `packetforward` (Linux)

Counts number of packets/bytes passing through the `eth0` interface of a Node, along with the direction of the packets.

## Metrics

See metrics for [Basic Mode](../modes/basic.md#plugin-packetforward-linux) (Advanced modes have identical metrics).

Note: `adv_forward_count` and `adv_forward_bytes` metrics are NOT associated with `packetforward` plugin despite similarities in name.
These metrics are associated with [`packetparser` plugin](./packetparser.md).

## Architecture

The plugin utilizes eBPF to gather data.
The plugin generates Basic metrics from an eBPF result.

### Code locations

- Plugin and eBPF code: *pkg/plugin/packetforward/*
