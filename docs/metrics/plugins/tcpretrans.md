# `tcpretrans` (Linux)

Measures retransmitted TCP packets.

## Metrics

See metrics for [Advanced Mode](../advanced.md#plugin-tcpretrans-linux).

## Architecture

The plugin utilizes eBPF to gather data.
The plugin does not generate Basic metrics.
In Advanced mode (see [Metric Modes](../modes.md)), the plugin turns an eBPF result into an enriched `Flow` (adding Pod information based on IP), then sends the `Flow` to an external channel so that a tcpretrans module can create Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/tcpretrans/*
- Module for extra Advanced metrics: *pkg/module/metrics/tcpretrans.go*
