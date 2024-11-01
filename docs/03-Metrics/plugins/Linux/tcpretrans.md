# `tcpretrans`

Measures retransmitted TCP packets.

## Capabilities

The `tcpretrans` plugin requires the `CAP_SYS_ADMIN` capability.

## Architecture

The plugin utilizes eBPF to gather data.
The plugin does not generate Basic metrics.
In Advanced mode (see [Metric Modes](../../modes/modes.md)), the plugin turns an eBPF result into an enriched `Flow` (adding Pod information based on IP), then sends the `Flow` to an external channel so that a tcpretrans module can create Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/tcpretrans/*
- Module for extra Advanced metrics: *pkg/module/metrics/tcpretrans.go*

## Metrics

See metrics for [Advanced Mode](../../modes/advanced.md#plugin-tcpretrans-linux).
