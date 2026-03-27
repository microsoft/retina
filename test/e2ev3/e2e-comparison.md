# E2E Framework Comparison: `test/e2e/` vs `test/e2ev3/`

## Architecture

| Aspect | `e2e/` (v1) | `e2ev3/` (v3) |
|---|---|---|
| **Orchestration** | Custom step/job/scenario runner (`framework/types`) | [Azure/go-workflow](https://github.com/Azure/go-workflow) DAG engine |
| **Execution model** | Sequential step list within a `Job`; scenarios grouped into jobs | Declarative `flow.Pipe` / `flow.BatchPipe` DAG with `DependsOn` |
| **Retry / backoff** | Manual retry in Prometheus helpers | First-class `Retry(k8s.RetryWithBackoff)` on any step |
| **Cleanup guarantee** | Convention-based (stop steps must be wired manually) | `When(flow.Always)` ensures cleanup runs after failures |
| **Background steps** | `RunInBackgroundWithID` + `Stop` pairing with validation | Not used; DAG dependencies replace background lifecycle |
| **Parameter passing** | Reflection-based auto-save of exported string fields into `JobValues` (immutable) | Shared `*config.E2EConfig` struct passed by pointer |
| **Sub-tests** | None (`TestE2ERetina` is monolithic) | None (`TestE2ERetina` is monolithic) |
| **Build tags** | `e2e`, `perf`, `scale` | `e2e` only |
| **Providers** | Azure only (Kind not natively supported) | Azure and Kind (native SDK) |
| **Logging** | Standard `testing.T` + Go log | Custom `slog` handler with color, caller detection, and per-step prefixes |

## Entry Points

| Entry Point | `e2e/` | `e2ev3/` |
|---|---|---|
| `TestE2ERetina` | ✅ `retina_e2e_test.go` | ✅ `retina_e2e_test.go` |
| `TestE2EPerfRetina` | ✅ `retina_perf_test.go` | ❌ Not implemented |
| `TestE2ERetina_Scale` | ✅ `scale_test.go` | ❌ Not implemented |

## Scenario Coverage

### Basic Metrics

| Scenario | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Drop metrics | ✅ | ✅ | deny-all NetworkPolicy, validate `drop_count` / `drop_bytes` |
| TCP metrics | ✅ | ✅ | TCP state, connection remote |
| DNS (valid domain) | ✅ | ✅ | `dns_request_count` / `dns_response_count` |
| DNS (NXDOMAIN) | ✅ | ✅ | non-existent domain |
| Windows HNS metrics | ✅ | ✅ | Skipped on Kind in v3 |

### Basic Metrics — Experimental (v3 only)

| Scenario | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Forward metrics | ❌ | ✅ | `forward_count` / `forward_bytes` |
| Conntrack metrics | ❌ | ✅ | `conntrack_packets_{tx,rx}`, `conntrack_bytes_{tx,rx}`, `conntrack_total_connections` |
| TCP stats | ❌ | ✅ | `tcp_connection_stats`, `tcp_flag_gauges` |
| Network stats | ❌ | ✅ | `ip_connection_stats`, `udp_connection_stats`, `interface_stats` |
| Node connectivity | ❌ | ✅ | `node_connectivity_status`, `node_connectivity_latency_seconds` |

### Advanced Metrics

| Scenario | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Advanced DNS (valid) | ✅ | ✅ | `adv_dns_request_count` / `adv_dns_response_count` |
| Advanced DNS (NXDOMAIN) | ✅ | ✅ | |
| API server latency | ✅ | ✅ | `adv_node_apiserver_tcp_handshake_latency` |

### Advanced Metrics — Experimental (v3 only)

| Scenario | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Advanced drop | ❌ | ✅ | `adv_drop_count` / `adv_drop_bytes` |
| Advanced forward | ❌ | ✅ | `adv_forward_count` / `adv_forward_bytes` |
| Advanced TCP | ❌ | ✅ | `adv_tcpflags_count` / `adv_tcpretrans_count` |
| Advanced API server latency | ❌ | ✅ | `adv_node_apiserver_latency` / `adv_node_apiserver_no_response` |

### Hubble Metrics

| Scenario | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Relay service validation | ✅ | ✅ | |
| UI service + HTTP check | ✅ | ✅ | |
| DNS queries / responses | ✅ | ✅ | |
| Flow intra-node | ✅ | ✅ | pod-to-pod same node |
| Flow inter-node | ✅ | ✅ | pod-to-pod different nodes |
| Flow to world | ✅ | ✅ | pod → external (bing.com) |
| Drop metrics | ✅ | ✅ | Hubble + Retina drop validation |
| TCP flags (SYN / FIN) | ✅ | ✅ | |

### Capture

| Scenario | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Capture create / download / delete | ✅ | ✅ | Full kubectl-retina capture lifecycle |

### Performance

| Scenario | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Perf benchmark (before Retina) | ✅ | ❌ | App Insights integration, throughput / RTT / jitter |
| Perf result (after Retina) | ✅ | ❌ | Delta computation and regression detection |

### Scale

| Scenario | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Scale test (mass deployments / policies) | ✅ | ❌ | Configurable pod/service/policy volume |
| Label mutation stress | ✅ | ❌ | delete-and-re-add labels |
| Metric collection at scale | ✅ | ❌ | App Insights telemetry |

## Infrastructure Providers

| Provider | `e2e/` | `e2ev3/` | Notes |
|---|---|---|---|
| Azure (AKS) | ✅ | ✅ | ARM templates in v3, `az` CLI wrapper in v1 |
| Kind (local) | ❌ | ✅ | Native Kind SDK, auto image sideloading |

## Framework Helpers (Kubernetes steps)

| Helper | `e2e/` | `e2ev3/` |
|---|---|---|
| Create/delete namespace | ✅ | ✅ |
| Create/delete generic resource | ✅ | ✅ |
| Label nodes | ✅ | ✅ |
| Network policy | ✅ | ✅ |
| Agnhost statefulset | ✅ | ✅ |
| Kapinger deployment | ✅ | ✅ |
| Exec pod | ✅ | ✅ |
| Port-forward | ✅ | ✅ |
| Install/upgrade/uninstall Helm | ✅ | ✅ |
| Validate service | ✅ | ✅ |
| Validate HTTP | ✅ | ✅ |
| Check pod status / no-crashes | ✅ | ✅ |
| Get pod IP | ✅ | ✅ |
| Get logs | ✅ | ✅ |
| Get external CRD | ✅ | ✅ |
| Debug on failure | ❌ | ✅ |
| Scale test resources | ✅ | ❌ |
| Scale network policies | ✅ | ❌ |
| Scale label operations | ✅ | ❌ |

## Configuration

| Config Source | `e2e/` | `e2ev3/` |
|---|---|---|
| Flags (`-provider`, `-create-infra`, etc.) | ✅ via `flag` + custom | ✅ via `flag` |
| Env vars via reflection | ✅ (auto-bind string fields) | ❌ |
| Env vars via `viper` | ❌ | ✅ (explicit binding map) |
| Metric constants | `framework/constants/` | `config/metrics.go`, `config/network.go` |

## Summary

**v3 advantages:**

- Kind provider for fast local development (no Azure credentials needed)
- DAG-based orchestration with built-in retry, timeout, and cleanup guarantees
- 10 new experimental metric scenarios not present in v1
- Richer logging with per-step prefixes and color
- ARM template-based Azure provisioning (replaces CLI wrappers)
- Debug-on-failure step for post-mortem pod log capture

**v1-only features not yet in v3:**

- Performance benchmarking (`perf` build tag, App Insights integration)
- Scale testing (`scale` build tag, mass deployment/policy creation)
- `retina-mode` flag for switching basic/advanced profiles in perf tests

**Parity:** All core metric scenarios (basic, advanced, hubble, capture) are present in both frameworks.
