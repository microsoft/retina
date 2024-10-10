# `ciliumEventObserverWindows`

Collect events from the Windows cilium eBPF event map and configure the event filter.

## DRAFT

This file documents the up-coming retina support for cilium on Windows.
The features described here aren't supported yet.

## Architecture

Windows cilium sends events to an eBPF ring buffer map at various points in the bpf programs.
The events supported include packet drop, trace, and policy verdicts. cilium capture events are also supported,
but currently only include the cilium event details (not the raw packet data).

The Windows cilium plugin will attach to the ring buffer and process events by decoding, parsing,
and passing them to the external channel as flow objects.
The retina daemon listens for these events and send it to our monitor agent.
Our hubble observer will consume these events and process the flows using our own custom
[parsers](https://github.com/microsoft/retina/tree/main/pkg/hubble/parser).

## Event filter configuration

On linux, cilium observability is configured by recompiling the bpf programs with different precompiler flags.
On Windows, to support HVCI the observability is configured at runtime instead through an event configuration map
stored in an eBPF array.

To configure the event filter in retina, modify the appropriate values in the `retina-config`
[ConfigMap](../02-Installation/03-Config.md).
When retina reloads the configuration it will write the corresponding values to the event configuration map.

_(DRAFT)_
```
cilium_windows:
  # NOTE: currently DROP events are always enabled
  enabled_events:
    - DROP
    - TRACE
    - POLICY_VERDICT
  # supported levels: none, low, medium, maximum
  monitor_aggregation_level: maximum
```

### Code locations

- Plugin and eBPF code: *pkg/plugin/ciliumeventobserver_windows/*

## Metrics

The supported retina metics are TBD.
