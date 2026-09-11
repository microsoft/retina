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

## Status

Implemented: the `ebpfwindows` plugin consumes flows from the WCN/eBPF-for-Windows
observability producer via a gRPC Observer stream over a node-local socket
(`ObserverSource` in `source.go`), the same mechanism the `pktmon` plugin uses.
Each incoming flow is normalized (`normalizeFlow` in `normalize.go`) so that the
WCN flows carry the verdict, traffic direction, and a drop reason extension that
Retina's advanced flow metrics read. Unit tests run an in-process Observer
server over a local socket (no WCN runtime required) and exercise the full
lifecycle, forwarding, drop, and nil-event paths.

## Open items

- Validate end-to-end against a real Windows Server 2025 + Cilium-on-Windows
  node where the WCN observability producer runs (not available in dev).
- Confirm/align the default socket path with the WCN deployable.
- Fidelity/parity review against the Linux dropreason / flow path.

## Metrics

Flows are mapped onto Retina's advanced flow metric model. `Verdict` (DROPPED /
FORWARDED) drives whether a flow is counted by the drop or forward metric
(`adv_drop_count` / `adv_forward_count`), `TrafficDirection` is the metric
`direction` label, and a DROPPED flow carries a `drop_reason` extension used for
the drop metric's `reason` label.
