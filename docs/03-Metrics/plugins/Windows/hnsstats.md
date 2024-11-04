# `hnsstats`

Gathers TCP statistics and counts number of packets/bytes forwarded or dropped in HNS and VFP.

## Architecture

Interfaces with a Windows Node's HNS (Host Networking System) and VFP (Virtual Filtering Platform).

### Code Locations

- Plugin code interfacing with HNS/VFP: *pkg/plugin/windows/hnsstats*

## Metrics

See metrics for [Basic Mode](../../modes/basic.md#plugin-hnsstats-windows) (Advanced modes have identical metrics).
