# Retina Rust

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/microsoft/retina/blob/main/LICENSE)

Rust rewrite of [Retina](https://github.com/microsoft/retina). Built with [aya-rs](https://github.com/aya-rs/aya) and fully compatible with the [Hubble](https://github.com/cilium/hubble) ecosystem — existing Cilium tooling (Hubble CLI, relay, UI) works out of the box, no Cilium installation required.

## Features

- **eBPF packet capture** — TC classifiers on host and pod interfaces, with conntrack-aware deduplication and probabilistic sampling
- **Hubble-compatible gRPC** — Observer and Peer services stream flows to Hubble relay, CLI, and UI without modification
- **Kubernetes identity enrichment** — flows are decorated with pod name, namespace, labels, and workload metadata from a centralized IP cache
- **Packet drop monitoring** — optional dropreason plugin traces `kfree_skb`, iptables, conntrack, and TCP drops via BTF-enabled tracepoints and fexit probes
- **Prometheus metrics** — forward/drop counters with full source/destination identity labels, plus agent health gauges
- **Adaptive kernel support** — auto-selects BPF ring buffer (kernel 5.8+) or per-CPU perf arrays; TCX attach (kernel 6.6+) or legacy TC filters
- **Whitelist/blacklist flow filtering** — Hubble-compatible filter compilation for live gRPC subscriptions
- **Debug endpoints** — runtime config, IP cache dump, memory breakdown, and CPU profiling (pprof) over HTTP
- **Helm chart** — single-command deployment of agent (DaemonSet), operator (Deployment), and optional Hubble relay + UI

## Architecture

The system consists of two binaries and a set of eBPF programs:

**retina-agent** runs as a DaemonSet on every node:
1. Loads eBPF TC classifiers (4 attach points: host ingress/egress, endpoint ingress/egress)
2. Reads `PacketEvent` structs from a BPF ring buffer or per-CPU perf array
3. Converts events to Hubble `Flow` protobufs and enriches with Kubernetes metadata
4. Broadcasts flows to gRPC subscribers and stores in a fixed-capacity ring buffer for historical queries
5. Serves Hubble Observer and Peer gRPC APIs, Prometheus metrics, and debug HTTP endpoints

**retina-operator** runs as a single-replica Deployment:
1. Watches Pods, Services, and Nodes via the Kubernetes API
2. Maintains a canonical IP-to-identity cache
3. Streams `IpCacheUpdate` messages to agents over gRPC

### Data Flow

```
eBPF TC classifiers ──→ ring buffer / perf array
                              │
                    event reader (1 or N threads)
                              │
                    PacketEvent → Flow protobuf
                              │
                    enrich with IP cache metadata
                              │
                   ┌──────────┴──────────┐
                   │                     │
           broadcast::Sender        FlowStore
           (live subscribers)     (historical ring)
                   │                     │
                   └──────────┬──────────┘
                              │
                    Hubble Observer gRPC
```

## Workspace Crates

| Crate | Path | Description |
|-------|------|-------------|
| `retina-agent` | `cmd/agent/` | DaemonSet binary — loads eBPF, collects packets, serves Hubble gRPC |
| `retina-operator` | `cmd/operator/` | Deployment binary — watches K8s API, maintains IP-to-identity cache |
| `retina-core` | `crates/core/` | Shared library — flow conversion, enrichment, filtering, metrics, stores |
| `retina-proto` | `crates/proto/` | Protobuf/gRPC code generated from `.proto` files via tonic |
| `retina-common` | `plugins/packetparser/common/` | `no_std` C-compatible structs shared between eBPF and userspace |
| `packetparser` | `plugins/packetparser/userspace/` | Plugin — eBPF loader, event reader, veth watcher, conntrack GC |
| `dropreason-common` | `plugins/dropreason/common/` | `no_std` types for dropreason eBPF programs |
| `dropreason` | `plugins/dropreason/userspace/` | Plugin — kfree_skb tracepoint, iptables/conntrack/TCP drop tracing |
| `xtask` | `xtask/` | Build orchestration — build-ebpf, image, deploy subcommands |

The eBPF crates (`plugins/*/ebpf/`) have their own workspaces and toolchain (`nightly-2025-12-01`) since they target `bpfel-unknown-none`.

## Build Requirements

- **Stable Rust** (via `rust-toolchain.toml`) for userspace code
- **Nightly Rust** (`nightly-2025-12-01`) + [`bpf-linker`](https://github.com/aya-rs/bpf-linker) for eBPF compilation
- **protoc** (protobuf compiler) for gRPC code generation
- System libraries: `elfutils-libelf-devel`, `zlib-devel`, `pkgconf-pkg-config`

### Installing bpf-linker

```bash
cargo +nightly-2025-12-01 install bpf-linker
```

## Building

eBPF programs must be built before userspace code (the agent embeds the eBPF binaries via `include_bytes!()`):

```bash
# Build eBPF programs (both perf and ringbuf variants, for all plugins)
cargo xtask build-ebpf --release

# Build both agent and operator binaries
cargo xtask build

# Build individually
cargo xtask build-agent --release
cargo xtask build-operator --release

# Build container images
cargo xtask image              # Both agent and operator
cargo xtask image-agent --tag mytag
cargo xtask image-operator --tag mytag
```

## Deploying

```bash
# Deploy everything (operator + agent + Hubble relay/UI)
cargo xtask deploy --cluster mycluster --namespace retina

# Deploy components individually
cargo xtask deploy-operator --cluster mycluster --namespace retina
cargo xtask deploy-agent --cluster mycluster --namespace retina
cargo xtask deploy-hubble --namespace retina
```

For kind clusters, xtask auto-detects and uses `kind load` instead of a container registry. For remote clusters, pass `--registry <registry>` to push images.

The Helm chart in `deploy/` creates a DaemonSet (agent), Deployment (operator), and optional Hubble relay + UI.

### Connecting Hubble

Once deployed, use the standard Hubble CLI to observe flows:

```bash
# Port-forward to agent gRPC (xtask can do this automatically for kind)
cargo xtask port-forward --namespace retina

# Observe live flows
hubble observe --server localhost:4244

# With filters
hubble observe --server localhost:4244 --namespace kube-system --protocol TCP
```

## Agent CLI Reference

```
retina-agent [OPTIONS]

Options:
    --interface <IFACE>           Host network interface for TC programs (omit for pod-level only)
    --pod-level                   Enable pod-level monitoring via veth endpoint programs
    --grpc-port <PORT>            Hubble Observer gRPC port [default: 4244]
    --operator-addr <URL>         Operator gRPC address for IP cache enrichment
    --sampling-rate <N>           Packet sampling rate: 1 = all, N = ~1/N [default: 1]
    --ring-buffer-size <BYTES>    BPF ring buffer size (power of 2) [default: 2097152]
    --enable-dropreason           Enable packet drop monitoring plugin
    --dropreason-ring-buffer-size <BYTES>  Ring buffer for drop events [default: 1048576]
    --dropreason-filter-path <PATH>        YAML filter config [default: /etc/retina/dropreason-filter.yaml]
    --log-level <LEVEL>           Log level [default: info]
    --metrics-port <PORT>         HTTP port for metrics/health/debug [default: 10093]
```

## Operator CLI Reference

```
retina-operator [OPTIONS]

Options:
    --grpc-port <PORT>    IpCache gRPC service port [default: 9090]
    --debug-port <PORT>   Debug HTTP port [default: 9091]
    --log-level <LEVEL>   Log level [default: info]
```

## Debug Endpoints

**Agent** (default `:10093`):

| Endpoint | Description |
|----------|-------------|
| `GET /metrics` | Prometheus metrics (OpenMetrics format) |
| `GET /healthz` | Liveness probe (event readers alive) |
| `GET /readyz` | Readiness probe (plugin started, gRPC bound, cache synced) |
| `GET /debug/config` | Runtime config and store stats |
| `GET /debug/ipcache` | Full IP cache dump |
| `GET /debug/mem` | Process memory breakdown (RSS, PSS, BPF maps, stacks) |
| `GET /debug/pprof/profile` | CPU profile (pprof format, `?seconds=30&frequency=1000`) |

**Operator** (default `:9091`):

| Endpoint | Description |
|----------|-------------|
| `GET /debug/ipcache` | Full IP cache dump |
| `GET /debug/stats` | Cache stats, broadcast queue depth, connected agents |

## Testing

```bash
cargo test --workspace                         # All tests
cargo test -p retina-core                      # Single crate
cargo test -p retina-core -- enricher          # Tests matching pattern
cargo clippy --workspace                       # Lint
cargo fmt --all -- --check                     # Format check
```

### Benchmarks

Criterion benchmarks in `crates/core/benches/` cover the hot path (8 targets: `flow_bench`, `enricher_bench`, `ipcache_bench`, `filter_bench`, `metrics_bench`, `store_bench`, `pipeline_bench`, `contention_bench`):

```bash
cargo bench --package retina-core                          # All benchmarks
cargo bench --package retina-core --bench flow_bench       # Single target

# Save and compare baselines
cargo bench --package retina-core --bench flow_bench -- --save-baseline v1
cargo bench --package retina-core --bench flow_bench -- --baseline v1
```

Note: `--save-baseline` / `--baseline` must be passed per `--bench` target (the lib test harness doesn't understand Criterion flags).

## eBPF Programs

### PacketParser

Located in `plugins/packetparser/ebpf/`. TC classifier programs attached to network interfaces that parse packets, run conntrack, apply probabilistic sampling, and emit `PacketEvent` structs (112 bytes) to userspace.

Key eBPF maps:
- `EVENTS` — ring buffer or perf array for event delivery
- `CONNTRACK` — LRU hash map (262K entries) tracking active connections
- `RETINA_CONFIG` — array map for runtime config (sampling rate)

### DropReason

Located in `plugins/dropreason/ebpf/`. BTF-enabled tracepoints and fexit probes that detect packet drops:
- `kfree_skb` tracepoint — kernel packet drops with reason codes (kernel 5.17+)
- `tcp_v4_connect` / `inet_csk_accept` fexit — TCP connection drops
- `nf_hook_slow` fexit — iptables/netfilter drops
- `__nf_conntrack_confirm` fexit — conntrack drops
- `tcp_retransmit_skb` / `tcp_send_reset` / `tcp_receive_reset` — TCP retransmits and resets

Key eBPF maps:
- `DROPREASON_EVENTS` — ring buffer or perf array for drop events
- `DROPREASON_METRICS` — per-CPU hash map for drop counters
- `DROPREASON_OFFSETS` — kernel struct field offsets resolved from BTF at startup
- `DROPREASON_SUPPRESS` — hash map for filtering out noisy drop reasons

### TC Attach Strategy

The loader attaches TC classifiers using a two-tier strategy for coexistence with other TC programs (e.g., Cilium):

- **TCX (kernel >= 6.6)**: Inserts at the head of the TCX chain via `LinkOrder::first()`, ensuring Retina runs before other programs. Returns `TC_ACT_UNSPEC` (`TCX_NEXT`) to continue the chain.
- **Legacy TC (kernel < 6.6)**: Falls back to netlink-based `cls_bpf` with priority 1. `TC_ACT_UNSPEC` causes `continue`, allowing subsequent filters to run.

This passive-observer design means Retina never affects packet verdicts.

### Event Delivery

The agent auto-selects the delivery mechanism based on kernel version:
- **BPF ring buffer** (kernel >= 5.8): Single shared buffer, single reader task. Better scaling on high-core systems.
- **Perf event array** (kernel < 5.8): Per-CPU buffers, one reader task per CPU. Automatic fallback.

Both variants are compiled by `cargo xtask build-ebpf` and embedded in the agent binary. The loader checks the kernel version at startup to select the appropriate variant.

## Protobuf Definitions

Source files in `crates/proto/proto/`:

| File | Services / Messages |
|------|-------------------|
| `flow/flow.proto` | `Flow`, `Endpoint`, `IP`, `Ethernet`, `TCP`, `UDP`, `EventType` |
| `observer/observer.proto` | `Observer` service — `GetFlows`, `GetAgentEvents`, `ServerStatus` |
| `peer/peer.proto` | `Peer` service — `Notify` (node discovery) |
| `relay/relay.proto` | `Relay` service (Hubble relay compatibility) |
| `ipcache/ipcache.proto` | `IpCache` service — `StreamIpCacheUpdates`, `GetIpCacheSnapshot` |

## Configuration

### Helm Values

Key values in `deploy/values.yaml`:

```yaml
agent:
  grpcPort: 4244           # Hubble Observer gRPC port
  metricsPort: 10093       # HTTP metrics/health/debug port
  podLevel: true           # Enable pod-level veth monitoring
  enableDropReason: true   # Enable packet drop monitoring
  dropReasonFilter:
    suppressedDropReasons: []  # Drop reasons to suppress from Hubble stream

operator:
  grpcPort: 9090           # IpCache gRPC service port
  debugPort: 9091          # Debug HTTP port

hubble:
  enabled: true            # Deploy Hubble relay + UI
```

### Drop Reason Filtering

The agent reads a YAML config to suppress noisy drop reasons from the Hubble event stream (metrics are still collected). Configure via `agent.dropReasonFilter.suppressedDropReasons` in the Helm values:

```yaml
agent:
  dropReasonFilter:
    suppressedDropReasons:
      - "NOT_SPECIFIED"
      - "QUEUE_PURGE"
```

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) in the repository root.

## License

This project is licensed under the [MIT License](../LICENSE).

Copyright (c) Microsoft Corporation.
