# `tcpretrans`

Measures retransmitted TCP packets.

## Capabilities

The `tcpretrans` plugin requires the `CAP_SYS_ADMIN` capability.

## Architecture

The plugin uses a native eBPF tracepoint (`tracepoint/tcp/tcp_retransmit_skb`) to capture TCP retransmission events. The BPF program extracts source/destination IPs, ports, and TCP flags, then streams events to user space via a perf buffer.

The plugin does not generate Basic metrics.
In Advanced mode (see [Metric Modes](../../modes/modes.md)), the plugin turns an eBPF result into an enriched `Flow` (adding Pod information based on IP), then sends the `Flow` to an external channel so that a tcpretrans module can create Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/tcpretrans/*
- BPF C source: *pkg/plugin/tcpretrans/_cprog/tcpretrans.c*
- Module for extra Advanced metrics: *pkg/module/metrics/tcpretrans.go*

## Metrics

See metrics for [Advanced Mode](../../modes/advanced.md#plugin-tcpretrans-linux).
