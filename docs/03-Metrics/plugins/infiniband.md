# `infiniband` (Linux)

Gathers Nvidia Infiniband port counters and debug status parameters from /sys/class/infiniband and /sys/class/net (respectively).

## Architecture

The plugin uses the following data sources:

1. `/sys/class/infiniband`
2. `/sys/class/net`

### Code Locations

- Plugin code interfacing with the Infiniband driver: *pkg/plugin/infiniband/*

## Metrics

- Infiniband Port Counter Statistics

- Infiniband Status Parameter Statistics

## Label Values for Infiniband Port Counters

Below is a running list of all statistics for Infiniband port counters

- `excessive_buffer_overrun_errors`
- `link_downed`
- `link_error_recovery`
- `local_link_integrity_errors`
- `port_rcv_constraint_errors`
- `port_rcv_data`
- `port_rcv_errors`
- `port_rcv_packets`
- `port_rcv_remote_physical_errors`
- `port_rcv_switch_replay_errors`
- `port_xmit_constraint_errors`
- `port_xmit_data`
- `port_xmit_discards`
- `port_xmit_packets`
- `symbol_error`
- `VL15_dropped`

## Label Values for Infiniband Debug Status Parameters

Below is a running list of all statistics for Infiniband debug status parameters

- `lro_timeout`
- `link_down_reason`
