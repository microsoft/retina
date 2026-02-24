# Retina Experimental — Rust eBPF Agent

Rust rewrite of [Retina](https://github.com/microsoft/retina)'s eBPF-based Kubernetes network observability agent. Uses [aya-rs](https://github.com/aya-rs/aya) for eBPF and produces Hubble-compatible gRPC streams, so existing Cilium tooling (Hubble CLI, relay, UI) works out of the box.

## Directory Layout

```
experimental/
├── cmd/
│   ├── agent/                  # retina-agent binary (DaemonSet)
│   └── operator/               # retina-operator binary (Deployment)
├── crates/
│   ├── core/                   # Shared library (flows, enrichment, filtering, metrics)
│   │   └── benches/            # Criterion benchmarks (8 targets)
│   └── proto/                  # Protobuf/gRPC definitions (tonic)
│       └── proto/              # .proto source files
├── plugins/
│   └── packetparser/
│       ├── common/             # no_std types shared between eBPF and userspace
│       ├── ebpf/               # eBPF TC classifier programs (nightly Rust)
│       └── userspace/          # Plugin: loader, event reader, veth watcher, conntrack GC
├── deploy/                     # Helm chart (agent, operator, Hubble relay/UI)
├── xtask/                      # Build orchestration (cargo xtask subcommands)
├── Dockerfile                  # Agent container image
├── Dockerfile.operator         # Operator container image
├── Cargo.toml                  # Workspace root
└── rust-toolchain.toml         # Stable Rust toolchain
```

## Workspace Crates

| Crate | Path | Description |
|-------|------|-------------|
| `retina-agent` | `cmd/agent/` | DaemonSet binary — loads eBPF, collects packets, serves Hubble gRPC |
| `retina-operator` | `cmd/operator/` | Deployment binary — watches K8s API, maintains IP-to-identity cache |
| `retina-core` | `crates/core/` | Shared library — flow conversion, enrichment, filtering, metrics, stores |
| `retina-proto` | `crates/proto/` | Protobuf/gRPC code generated from `.proto` files via tonic |
| `retina-common` | `plugins/packetparser/common/` | `no_std` C-compatible structs (`PacketEvent`, `CtV4Key`, `CtEntry`) |
| `packetparser` | `plugins/packetparser/userspace/` | Plugin — eBPF loader, event reader, veth watcher, conntrack GC |
| `xtask` | `xtask/` | Build helper — build-ebpf, image, deploy subcommands |

The eBPF crate (`plugins/packetparser/ebpf/`) has its own workspace and toolchain (`nightly-2025-12-01`) since it targets `bpfel-unknown-none`.

## Architecture

### Two Executables

**retina-agent** runs as a DaemonSet on every node:
1. Loads eBPF TC classifiers (4 attach points: host ingress/egress, endpoint ingress/egress)
2. Reads `PacketEvent` structs from a BPF ring buffer (kernel 5.8+) or per-CPU perf array (older kernels)
3. Converts events to Hubble `Flow` protobufs and enriches with K8s metadata from the IP cache
4. Broadcasts flows via `tokio::sync::broadcast` and stores in a fixed-capacity ring buffer
5. Serves Hubble Observer gRPC (live + historical flows) and Peer gRPC (node discovery)

**retina-operator** runs as a single-replica Deployment:
1. Watches Pods, Services, and Nodes via the K8s API
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

## Key Source Files

### Agent (`cmd/agent/src/`)

| File | Responsibility |
|------|---------------|
| `main.rs` | CLI args, plugin init, gRPC + debug server startup |
| `grpc.rs` | Hubble Observer (GetFlows, GetAgentEvents) and Peer (Notify) services |
| `ipcache_sync.rs` | Connects to operator, receives IP cache updates, marks cache synced |
| `debug.rs` | HTTP endpoints: `/metrics`, `/healthz`, `/readyz`, `/debug/*` |

### Operator (`cmd/operator/src/`)

| File | Responsibility |
|------|---------------|
| `main.rs` | CLI args, K8s client init, watcher + gRPC startup, graceful shutdown |
| `state.rs` | `OperatorState` — IP cache with broadcast, metrics counters |
| `watchers.rs` | K8s watches for Pods, Services, Nodes; upsert/delete identity entries |
| `grpc.rs` | IpCache gRPC service (snapshot + streaming updates) |
| `debug.rs` | HTTP endpoints: `/debug/ipcache`, `/debug/stats` |

### Core Library (`crates/core/src/`)

| File | Responsibility |
|------|---------------|
| `flow.rs` | `packet_event_to_flow()` — converts eBPF events to Hubble Flow protobufs |
| `enricher.rs` | `enrich_flow()` — adds namespace, pod name, labels, workload info from IP cache |
| `filter.rs` | `FlowFilterSet` — compiles whitelist/blacklist filters, matches flows |
| `ipcache.rs` | `IpCache` — thread-safe IP → `Identity` map with broadcast notifications |
| `metrics.rs` | Prometheus metrics registry and `AgentState` health tracking |
| `store.rs` | `FlowStore` / `AgentEventStore` — fixed-capacity ring buffers |
| `plugin.rs` | `Plugin` trait — `start(ctx)` / `stop()` lifecycle |
| `retry.rs` | Exponential backoff retry for gRPC connections |

### PacketParser Plugin (`plugins/packetparser/userspace/src/`)

| File | Responsibility |
|------|---------------|
| `plugin.rs` | Plugin entry point — loads eBPF, starts event reader, veth watcher, conntrack GC |
| `loader.rs` | eBPF ELF loader — kernel version detection, ringbuf vs perf selection, TC attach (TCX / legacy), `Align8` wrapper |
| `events.rs` | Event reader — perf (one OS thread per CPU) or ring buffer (single thread) |
| `watcher.rs` | Veth watcher — netlink monitoring for pod network interfaces, TC program attachment |
| `conntrack_gc.rs` | Conntrack garbage collector — sweeps expired entries every 15s |

## Protobuf Definitions

Source files in `crates/proto/proto/`:

| File | Services / Messages |
|------|-------------------|
| `flow/flow.proto` | `Flow`, `Endpoint`, `IP`, `Ethernet`, `TCP`, `UDP`, `EventType` |
| `observer/observer.proto` | `Observer` service — `GetFlows`, `GetAgentEvents`, `ServerStatus` |
| `peer/peer.proto` | `Peer` service — `Notify` (node discovery) |
| `relay/relay.proto` | `Relay` service (Hubble relay compatibility) |
| `ipcache/ipcache.proto` | `IpCache` service — `StreamIpCacheUpdates`, `GetIpCacheSnapshot` |

## TC Attach Strategy

The agent attaches TC classifiers using a two-tier strategy to coexist with other TC programs (e.g. Cilium) on the same interfaces:

- **TCX (kernel >= 6.6)**: Uses `LinkOrder::first()` to insert Retina at the head of the TCX chain, ensuring it runs before any other TC programs. All eBPF programs return `TC_ACT_UNSPEC` (`TCX_NEXT`), so the chain continues to subsequent programs unimpeded.
- **Legacy TC (kernel < 6.6)**: Falls back to netlink-based `cls_bpf` filters with priority 1 (lowest usable value). `TC_ACT_UNSPEC` causes `continue` in `cls_bpf`, allowing subsequent filters to run. Ordering within the same priority depends on attachment order.

This passive-observer design means Retina never affects packet verdicts — it observes and passes through, letting other programs (network policy enforcement, etc.) make the final decision.

## eBPF Programs

Located in `plugins/packetparser/ebpf/`. Two compilation variants:

- **Perf** (default): per-CPU `PerfEventArray`, one reader thread per CPU
- **Ring buffer** (`--features ringbuf`): single shared `RingBuf` (kernel 5.8+), single reader thread

Both are built by `cargo xtask build-ebpf` and embedded into the agent binary via `include_bytes!()`. The loader auto-selects based on kernel version at startup.

Key eBPF maps:
- `EVENTS` — event delivery (ring buffer or perf array)
- `CONNTRACK` — LRU hash map (262K entries) tracking active connections
- `RETINA_CONFIG` — array map for runtime config (sampling rate)

## Building

```bash
# eBPF (requires nightly + bpf-linker)
cargo xtask build-ebpf --release

# Userspace binaries
cargo xtask build --release          # Both agent and operator
cargo xtask build-agent --release    # Agent only
cargo xtask build-operator --release # Operator only

# Container images
cargo xtask image --tag mytag
```

## Testing

```bash
cargo test --workspace                       # All tests
cargo test -p retina-core                    # Single crate
cargo test -p retina-core -- enricher        # Tests matching pattern
cargo clippy --workspace                     # Lint
cargo fmt --all -- --check                   # Format check
```

### Benchmarks

Criterion benchmarks in `crates/core/benches/` (8 targets):

```bash
cargo bench --package retina-core                        # All benchmarks
cargo bench --package retina-core --bench flow_bench     # Single target
cargo bench --package retina-core --bench flow_bench -- --save-baseline v1
cargo bench --package retina-core --bench flow_bench -- --baseline v1
```

## Deploying

```bash
cargo xtask deploy-agent --cluster mycluster --namespace retina
cargo xtask deploy-operator --cluster mycluster --namespace retina
cargo xtask deploy-hubble --namespace retina
```

The Helm chart in `deploy/` creates a DaemonSet (agent), Deployment (operator), and optional Hubble relay + UI. For kind clusters, xtask auto-detects and uses `kind load` instead of a registry.

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

## Build Requirements

- **Stable Rust** (via `rust-toolchain.toml`) for userspace code
- **Nightly Rust** (`nightly-2025-12-01`) + `bpf-linker` for eBPF compilation
- **protoc** for gRPC code generation
- System libs: `elfutils-libelf-devel`, `zlib-devel`, `pkgconf-pkg-config`
