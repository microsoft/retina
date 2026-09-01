# `ebpfwindows`

> **Status: Work-in-progress (proposal).** This plugin is a compiling skeleton that
> expresses the intended design for sourcing Windows node observability from the
> eBPF-for-Windows data plane. The live consumer is not yet implemented.

Gathers network telemetry on a Windows node from the [eBPF-for-Windows](https://github.com/microsoft/ebpf-for-windows) data plane and the WCN/Cilium-on-Windows observability surface, as an alternative to the legacy HNS/VFP hnsstats path.

## Motivation

Retina's Linux plugins rely on eBPF to collect high-fidelity flow, drop-reason and packet-forward telemetry. On Windows today, observability is limited to node-level TCP/drop counts via HNS/VFP (hnsstats). As the Windows datapath moves to the WCN/Cilium-on-Windows architecture backed by eBPF-for-Windows, this plugin provides a home for a native eBPF-backed telemetry source on Windows, keeping Retina's flow objects as the portability boundary between platforms.

## Architecture

Interfaces with the eBPF-for-Windows runtime on a Windows node.

### Intended data flow

eBPF-for-Windows maps / ring buffers -> wcnagent / Microsoft.Wcn.Observability.eBPF adapter -> ebpfwindows plugin (Start) -> Retina flow objects -> metrics / control plane

### Code Locations

- Plugin code: pkg/plugin/ebpfwindows
- Registration: pkg/plugin/include_windows.go

## Status / Open items

- Implement the eBPF-for-Windows/WCN event consumer in Start.
- Convert raw events to cilium v1.Event flows (5-tuple, direction, verdict, duration).
- Wire the downstream channel (SetupChannel) to metrics and the Hubble observer.
- Validate against Windows Server 2025 with Cilium-on-Windows enabled.
- Fidelity/parity review against the Linux dropreason / flow path.

## Metrics

TBD until the consumer is implemented; intended to mirror the Linux l4/flow metrics.
