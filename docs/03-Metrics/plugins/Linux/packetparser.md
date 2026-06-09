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

### Performance Impact on High-Core-Count Systems

The `packetparser` eBPF data path has measurable overhead that varies by CPU core count, packet event rate, payload profile, and NUMA topology.

External analysis was published in [this blog post](https://blog.zmalik.dev/p/who-will-observe-the-observability) and presented at [KubeCon 2025](https://www.youtube.com/watch?v=J-Zx64mJzVk). Retina maintainers also ran internal benchmarking to compare `perf_event_array` and ring buffer transport.

**Scope note:** Results below are directional, not universal. Validate with your own workload.

#### Default Implementation: `perf_event_array`

By default, `packetparser` uses **BPF_MAP_TYPE_PERF_EVENT_ARRAY** for kernel-to-userspace transfer. This creates per-CPU buffers drained by userspace.

Observed overhead characteristics:

- CPU cost generally tracks packet event rate
- Cross-NUMA polling can add latency and CPU overhead on multi-socket systems
- Context-switch overhead often grows with core count under sustained high packet rates

#### Ring Buffer Alternative: `BPF_MAP_TYPE_RINGBUF`

`packetparser` also supports **BPF_MAP_TYPE_RINGBUF** (Linux kernel 5.8+):

```yaml
# In Retina agent ConfigMap
packetParserRingBuffer: enabled
packetParserRingBufferSize: 8388608  # 8 MiB default
```

Compared with per-CPU perf buffers, ring buffer mode uses a shared buffer model and often reduces CPU overhead for packet-heavy workloads. In many tests it also improves throughput stability.

#### Benchmark Results

All tests used Retina `v1.2.0` on AKS with Azure overlay CNI, `dataAggregationLevel: low`, `enablePodLevel: true`, and plugins `linuxutil, packetforward, packetparser, dns, dropreason`.

Rather than listing each run in detail, the key customer-facing observations are:

| Observation | What we saw | Decision impact |
|-------------|-------------|-----------------|
| CPU scaling at high packet rates | Ring buffer reduced Retina CPU by ~1.5x to 6.0x versus perf array as event rate increased (~2M to ~7.5M events/s) | Prefer ring buffer for packet-heavy and bursty workloads |
| Throughput stability | Under stress, perf array showed larger throughput variance while ring buffer was more stable in tested profiles | Prefer ring buffer when stability is as important as peak throughput |
| Workload sensitivity | Ring buffer was not always better (for example, one `grpc-streaming` profile favored perf array) | Benchmark both modes when traffic is medium-rate or protocol-specific |
| Low-rate / large-payload traffic | Differences were often negligible near NIC-saturation or low event rates | Either mode can be acceptable; choose simpler operationally |
| Topology effects | Multi-socket / high-core environments are more exposed to perf-array polling costs | Ring buffer is generally the safer default on 64+ vCPU or multi-NUMA nodes |

Summary: ring buffer is usually the best default for high event-rate production workloads, but final selection should be validated against your own SLO and traffic profile.

#### Practical Guidance

| Workload profile | Preferred mode | Why |
|------------------|----------------|-----|
| High packet rate (>2M events/s), bursty services, service mesh traffic | Ring buffer | Often lower Retina CPU and better stability |
| Multi-socket or large-core systems (64+ vCPU) | Ring buffer | Avoids per-CPU cross-NUMA polling |
| Low packet rate / large payload throughput | Either | Differences are usually small |
| Workload where ring buffer underperforms in your tests | Perf array | Use workload-specific validation |

#### Quick Decision Framework

Use these three inputs from production-like traffic:

- Packet event rate at peak (p95)
- Node topology (single NUMA vs multi-NUMA / 64+ vCPU)
- Observability budget (`retina-agent` CPU and tolerated telemetry sample drops)

Quick estimator:

$$pps \approx \frac{throughput\ (bytes/s)}{avg\ packet\ size\ (bytes)}$$

| Condition | Choose |
|-----------|--------|
| Linux kernel < 5.8 | Perf array |
| p95 event rate > 2M/s, bursty traffic, or 64+ vCPU / multi-socket | Ring buffer |
| < 1M events/s with large payloads and stable throughput | Either (benchmark) |
| Ring buffer shows higher CPU or worse throughput in your workload | Perf array |

Minimal validation (recommended):

1. Run 10 minutes with perf array and 10 minutes with ring buffer on the same node pool.
2. Compare throughput median/p95, throughput variance, and `retina-agent` CPU peak.
3. Pick the mode that meets SLO with lower observability CPU; if ring buffer drops samples, increase `packetParserRingBufferSize` to `16777216` and retest.

#### Recommended Starting Configuration

| Environment | `packetParserRingBuffer` | `packetParserRingBufferSize` |
|-------------|--------------------------|------------------------------|
| Standard (<=32 vCPU, moderate load) | `enabled` | `8388608` (8 MiB) |
| High-core-count (64+ vCPU) | `enabled` | `8388608` (8 MiB) |
| Extreme packet rate with sustained bursts | `enabled` | `16777216` (16 MiB) |

Note on ring buffer drops: under extreme sustained observability load, ring buffer may drop telemetry samples (not network packets). If this occurs, increase `packetParserRingBufferSize` and re-measure.

#### If You Experience Performance Issues

1. Enable ring buffer first:

	```yaml
	packetParserRingBuffer: enabled
	packetParserRingBufferSize: 8388608
	```

2. Enable sampling: set `dataSamplingRate` (see [Sampling](#sampling))
3. Use high data aggregation: configure `high` [data aggregation](../../../05-Concepts/data-aggregation.md)
4. Disable `packetparser` if needed: use Basic metrics mode
5. Monitor impact: track `retina-agent` CPU, throughput stability, and sample-drop behavior

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
