# `dns` (Linux)

Counts number of packets/bytes dropped on a Node, along with the direction and reason for drop.

## Metrics

See metrics for [Basic Mode](../basic.md#plugin-dns-linux) or [Advanced Mode](../advanced.md#plugin-dns-linux).

## Architecture

The plugin utilizes eBPF to gather data.
The plugin generates Basic metrics from an eBPF result.
In Advanced mode (see [Metric Modes](../modes.md)), the plugin turns this eBPF result into an enriched `Flow` (adding Pod information based on IP), then sends the `Flow` to an external channel so that a dns module can create extra Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/dns/*
- Module for extra Advanced metrics: *pkg/module/metrics/dns.go*
