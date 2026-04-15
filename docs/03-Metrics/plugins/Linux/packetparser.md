# `packetparser`

Captures TCP and UDP packets traveling to and from pods and nodes.

## Capabilities

The `packetparser` plugin requires the `CAP_NET_ADMIN` and `CAP_SYS_ADMIN` capabilities.

- `CAP_SYS_ADMIN` is used to load maps and programs into the kernel and assign them to user-defined structs - `LoadAndAssign()` method at `packetparser_linux.go:147`
- `CAP_NET_ADMIN` is used for the queuing discipline kernel mechanism - `getQdisc()` method at `packetparser_linux.go:430`

## Architecture

`packetparser` attached a [`qdisc` (Queuing Discipline)](https://www.man7.org/linux/man-pages/man8/tc.8.html) of type `clsact` to each pod's virtual interface (`veth`) and the host's default interface (`device`). This setup enabled the attachment of eBPF filter programs for both ingress and egress directions, allowing `packetparser` to capture individual packets traveling to and from the interfaces.

`packetparser` does not produce Basic metrics. In Advanced mode (refer to [Metric Modes](../../modes/modes.md)), the plugin transforms an eBPF result into an enriched `Flow` by adding Pod information based on IP. It then sends the `Flow` to an external channel, enabling *several modules* to generate Pod-Level metrics.

## Performance Considerations

### Reported Performance Impact on High-Core-Count Systems

Community users have reported performance considerations when running the `packetparser` plugin on systems with high CPU core counts (32+ cores) under sustained network load. While these reports have not been independently verified by the Retina maintainers, we document them here for awareness.

**User-Reported Observations:**

A detailed analysis by a Retina user (see [this blog post](https://blog.zmalik.dev/p/who-will-observe-the-observability)) and [KubeCon 2025 talk](https://www.youtube.com/watch?v=J-Zx64mJzVk) documented performance degradation that scaled non-linearly with CPU core count on nodes running network-intensive, multi-threaded workloads.

**Current Implementation:**

By default, `packetparser` uses **BPF_MAP_TYPE_PERF_EVENT_ARRAY** for kernel-to-userspace data transfer. This architecture creates per-CPU buffers that must be polled by a single reader thread. On systems with many CPU cores, this can lead to:

- Increased context switching overhead
- Memory access patterns that may not scale linearly
- Potential NUMA-related penalties on multi-socket systems

**Alternative Approaches:**

Alternative data transfer mechanisms like BPF ring buffers (BPF_MAP_TYPE_RINGBUF, available in Linux kernel 5.8+) use a shared buffer architecture that may perform better on high-core-count systems. Retina supports ring buffers for `packetparser` via `packetParserRingBuffer=enabled` and `packetParserRingBufferSize`.

**Note:** Ring buffer mode requires Linux kernel 5.8 or newer.

#### If You Experience Performance Issues

If you observe performance degradation on high-core-count nodes:

1. **Disable `packetparser`**: Use Basic metrics mode which doesn't require this plugin
2. **Enable Sampling**: Use the `dataSamplingRate` configuration option (see [Sampling](#sampling) section)
3. **Use High Data Aggregation**: Configure `high` [data aggregation](../../../05-Concepts/data-aggregation.md)
4. **Monitor Impact**: Watch for elevated CPU usage, context switches, or throughput changes

**Note:** The Retina team is evaluating options for addressing reported performance concerns, including potential support for alternative data transfer mechanisms. Community feedback and contributions are welcome.

## Sampling

Since `packetparser` produces many enriched `Flow` objects it can be quite expensive for user space to process.  Thus, when operating in `high` [data aggregation](../../../05-Concepts/data-aggregation.md) level optional sampling for reported packets is available via the `dataSamplingRate` configuration option.

`dataSamplingRate` is expressed in 1 out of N terms, where N is the `dataSamplingRate` value.  For example, if `dataSamplingRate` is 3 1/3rd of packets will be sampled for reporting.

Keep in mind that there are cases where reporting will happen anyways as to ensure metric accuracy.

### Code locations

- Plugin and eBPF code: *pkg/plugin/packetparser/*
- Modules for extra Advanced metrics: see section below.

## Metrics

See metrics for [Advanced Mode](../../modes/advanced.md#plugin-packetparser-linux). For module information, see [below](#modules).

### Modules

#### Module: forward

Code path: *pkg/module/metrics/forward.go*

Metrics produced:

- `adv_forward_count`
- `adv_forward_bytes`

#### Module: tcpflags

Code path: *pkg/module/metrics/tcpflags.go*

Metrics produced:

- `adv_forward_count`
- `adv_forward_bytes`

#### Module: latency (API Server)

Code path: *pkg/module/metrics/latency.go*

Metrics produced:

- `adv_node_apiserver_latency`
- `adv_node_apiserver_no_response`
- `adv_node_apiserver_tcp_handshake_latency`
