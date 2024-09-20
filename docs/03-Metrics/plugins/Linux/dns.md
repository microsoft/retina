# `dns`

Tracks incoming and outgoing DNS traffic, providing various metrics and details about the DNS queries and responses.

## Architecture

This plugin uses [Inspektor Gadget](https://github.com/inspektor-gadget/inspektor-gadget)'s DNS Tracer to track DNS traffic and generate basic metrics derived from the captured events.

In [Advanced mode](https://retina.sh/docs/metrics/modes), the plugin further processes the capture results into an enriched Flow with additional Pod information. Subsequently, the Flow is transmitted to an external channel. This allows a DNS module to generate additional Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/dns/*
- Module for extra Advanced metrics: *pkg/module/metrics/dns.go*

## Metrics

See metrics for [Basic Mode](../../modes/basic.md#plugin-dns-linux) or [Advanced Mode](../../modes/advanced.md#plugin-dns-linux).
