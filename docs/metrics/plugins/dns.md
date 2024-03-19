# `dns` (Linux)

Tracks incoming and outgoing DNS traffic, providing various metrics and details about the DNS queries and responses.

## Metrics

See metrics for [Basic Mode](../basic.md#plugin-dns-linux) or [Advanced Mode](../advanced.md#plugin-dns-linux).

## Architecture

This plugin fundamentally relies on [Inspektor Gadget](https://github.com/inspektor-gadget/inspektor-gadget)'s DNS Tracer for monitoring DNS traffic. It uses eBPF (Extended Berkeley Packet Filter) to efficiently track DNS events. Following the capture of these events, the plugin generates basic metrics derived from the eBPF results.

In its Advanced mode (refer to [Metric Modes](https://retina.sh/docs/metrics/modes) for more details), the plugin further processes the eBPF results into an enriched Flow. This Flow includes additional Pod information, determined by IP. Subsequently, the Flow is transmitted to an external channel. This allows a DNS module to generate additional Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/dns/*
- Module for extra Advanced metrics: *pkg/module/metrics/dns.go*
