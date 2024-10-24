# `dropreason`

Counts number of packets/bytes dropped on a Node, along with the direction and reason for drop.

## Capabilities

The `dropreason` plugin requires the `CAP_SYS_ADMIN` and the `CAP_DAC_OVERRIDE` capabilities.
- `CAP_DAC_OVERRIDE` is used to override discretionary access control (DAC) restrictions, enabling the process to read, write, or execute files it would not normally have permissions to access - `LoadAndAssign()` method at `dropreason_linux.go:135`
- `CAP_SYS_ADMIN` is used to increase the memory lock limits, enabling the allocation of additional memory for eBPF programs and maps - `LoadAndAssign()` method at `dropreason_linux.go:135`

## Architecture

The plugin utilizes eBPF to gather data.
The plugin generates Basic metrics from an eBPF result.
In Advanced mode (see [Metric Modes](../../modes/modes.md)), the plugin turns this eBPF result into an enriched `Flow` (adding Pod information based on IP), then sends the `Flow` to an external channel so that a drops module can create extra Pod-Level metrics.

### Code locations

- Plugin and eBPF code: *pkg/plugin/dropreason/*
- Module for extra Advanced metrics: *pkg/module/metrics/drops.go*

## Metrics

See metrics for [Basic Mode](../../modes/basic.md#plugin-dropreason-linux) or [Advanced Mode](../../modes/advanced.md#plugin-dropreason-linux).

### Data sources

This plugin reads data from variable eBPF progs writing into the same eBPF map called `metrics_map`.
It aggregates data for basic metrics at a node level and then control plane of the plugin converts into Prometheus metrics.

See *pkg/plugin/dropreason/drop_reason.c*:

```c
struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_HASH);
    __uint(max_entries, 512);
    __type(key, struct metrics_map_key);
    __type(value, struct metrics_map_value);
} retina_dropreason_metrics SEC(".maps");

struct metrics_map_key
{
    __u32 drop_type;
    __u32 return_val;
};

struct metrics_map_value
{
    __u64 count;
    __u64 bytes;
};

```

### eBPF Hook Points for Drop Reason

| Reason | Description | eBPF Hook Point |
|--|--| -- |
| IPTABLE_RULE_DROP | Packets dropped by iptables rule | `kprobe/nf_hook_slow` |
| IPTABLE_NAT_DROP | Packets dropped by iptables NAT rule | `kretprobe/nf_nat_inet_fn` |
| TCP_CONNECT_BASIC | Packets dropped by TCP connect | `kretprobe/tcp_v4_connect` |
| TCP_ACCEPT_BASIC | Packets dropped by TCP accept | `kretprobe/inet_csk_accept` |
| TCP_CLOSE_BASIC | Packets dropped by TCP close | TBD |
| CONNTRACK_ADD_DROP | Packets dropped by conntrack add | `kretprobe/__nf_conntrack_confirm` |
| UNKNOWN_DROP | Packets dropped by unknown reason | NA |

This list will keep on growing as we add support for more reasons.
