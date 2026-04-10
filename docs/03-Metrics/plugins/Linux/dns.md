# `dns`

Tracks incoming and outgoing DNS traffic, providing various metrics and details about the DNS queries and responses.

## Capabilities

The `dns` plugin requires the `CAP_SYS_ADMIN` capability.

- `CAP_SYS_ADMIN` is used to load and attach the eBPF socket filter program

## Architecture

The plugin uses a native eBPF socket filter attached to an `AF_PACKET` socket to capture DNS queries and responses. The BPF program extracts source/destination IPs, ports, and query type, then streams events to user space via a perf buffer. Go-side parsing uses `gopacket` for DNS name and response address extraction.

In [Advanced mode](https://retina.sh/docs/Metrics/modes), the plugin further processes the capture results into an enriched Flow with additional Pod information. Subsequently, the Flow is transmitted to an external channel. This allows a DNS module to generate additional Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/dns/*
- BPF C source: *pkg/plugin/dns/_cprog/dns.c*
- Module for extra Advanced metrics: *pkg/module/metrics/dns.go*

## Metrics

See metrics for [Basic Mode](../../modes/basic.md#plugin-dns-linux) or [Advanced Mode](../../modes/advanced.md#plugin-dns-linux).
