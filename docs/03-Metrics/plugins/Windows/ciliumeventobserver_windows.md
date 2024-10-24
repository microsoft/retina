# `ciliumEventObserverWindows`

Collect events from the Windows cilium eBPF event map and configure the event filter.

## DRAFT

This file documents the up-coming retina support for cilium on Windows.
The features described here aren't supported yet.

## Architecture

Windows cilium sends events to an eBPF ring buffer map at various points in the bpf programs.
The events supported include packet drop, trace, and policy verdicts. cilium trace capture events are also supported,
but currently only include the cilium event details (not the raw packet data).

To connect to these events you can use the `ciliumeventobserver_windows` retina plugin.
The Windows cilium plugin will attach to the `cilium_events` ring buffer and read the
the events, then decode and parse the events and wrap them in a flow object.
The flow object will then be sent to the external channel for processing.

### Processing Pipeline

1. Cilium bpf program writes events to ring buffer
2. Windows cilium retina agent plugin will:
   1. collect the ring buffer events
   2. Decode and parse the events into Hubble-compatible flow objects
   3. Write the flow objects to the external channel
3. From the external channel retina will process the events into metrics.

### Cilium generated events

The Windows version of cilium writes observability and debugging events to an eBPF ring buffer map.
The event structures used in the Windows cilium eBPF programs have the same definitions as on linux,
so can use the same decoding logic. The main difference in the Windows eBPF program is the runtime [Event filtering](#event-filter-configuration), where instead of conditionally compiling in the observability reporting
The cilium programs check an observability configuration array map at runtime to decide whether to emit events.


### Collecting cilium events

The cilium events map on windows is a ring buffer map.
[cilium/ebpf-go](https://github.com/cilium/ebpf) does not yet run on Windows,
so we will add a small golang library to connect to the ebpf-for-windows ring buffer.

### Event Parsing

The raw event parsing will be done using the existing [cilium/cilium/pkg/monitor](https://github.com/cilium/cilium/tree/main/pkg/monitor), since cilium on windows has the same binary event format.

### Event Processing

The retina daemon listens for the flow objects the plugin emits and sends them to our monitor agent.
Our hubble observer will consume these events and process the flows using our own custom
[parsers](https://github.com/microsoft/retina/tree/main/pkg/hubble/parser).

## Event filter configuration

On linux, cilium observability is configured by recompiling the bpf programs with different precompiler flags,
e.g. `DEBUG`/`TRACE_NOTIFY`.
On Windows, to support HVCI the observability is configured at runtime instead through an event configuration map
stored in an eBPF array `cilium_observability_config`.

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

### Event types

- DROP - emitted whenever a packet is dropped (these events are currently always enabled)
- TRACE - emitted whenever a packet is forwarded by one of the cilium ebpf programs
- POLICY_VERDICT - reports network policy decisions (allow/drop with reason for drops)
- DEBUG - events emitted by the cilium code for debugging

### Monitor aggregation

The monitor aggregation value has the same meaning as the linux cilium monitor aggregation level:

- **none**: no filtering - report all enabled events
- **low**: only report TX events (filter out RX events)
- **medium**/**maxmimum**: only report each connection once per aggregation interval for a given set of flags

## Code locations

- Plugin and eBPF code: *pkg/plugin/ciliumeventobserver_windows/*

## Metrics

The supported retina metics are TBD.
