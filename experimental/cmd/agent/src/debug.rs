use std::sync::Arc;

use axum::Router;
use axum::extract::{Query, State};
use axum::http::{HeaderValue, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use prometheus_client::encoding::text::encode;
use prost_pprof::Message as _;
use retina_core::ipcache::IpCache;
use retina_core::metrics::{AgentState, Metrics};
use retina_core::store::FlowStore;
use serde::{Deserialize, Serialize};
use tokio::fs;
use tracing::info;

use crate::AgentConfig;

#[derive(Clone)]
struct DebugState {
    config: AgentConfig,
    ip_cache: Arc<IpCache>,
    flow_store: Arc<FlowStore>,
    metrics: Arc<Metrics>,
    state: Arc<AgentState>,
}

#[derive(Deserialize)]
struct ProfileParams {
    seconds: Option<u64>,
    frequency: Option<i32>,
}

#[derive(Serialize)]
struct ConfigDump {
    config: AgentConfig,
    ip_cache: IpCacheStats,
    flow_store: FlowStoreStats,
}

#[derive(Serialize)]
struct IpCacheStats {
    entries: usize,
    synced: bool,
}

#[derive(Serialize)]
struct FlowStoreStats {
    buffered_flows: u64,
    total_flows_seen: u64,
    capacity: usize,
    uptime_secs: f64,
}

#[derive(Serialize)]
struct MemInfo {
    rss_kb: u64,
    rss_anon_kb: u64,
    rss_file_kb: u64,
    vm_size_kb: u64,
    vm_peak_kb: u64,
    vm_data_kb: u64,
    vm_stk_kb: u64,
    threads: u64,
    smaps: SmapsRollup,
    maps_summary: MapsSummary,
}

#[derive(Serialize, Default)]
struct SmapsRollup {
    pss_kb: u64,
    pss_anon_kb: u64,
    pss_file_kb: u64,
    shared_clean_kb: u64,
    private_dirty_kb: u64,
    swap_kb: u64,
    anon_huge_pages_kb: u64,
}

#[derive(Serialize, Default)]
struct MapsSummary {
    total_regions: usize,
    perf_event_rings: usize,
    perf_event_kb: u64,
    ring_buf_regions: usize,
    ring_buf_kb: u64,
    bpf_map_regions: usize,
    bpf_map_kb: u64,
    heap_kb: u64,
    anonymous_kb: u64,
    binary_kb: u64,
    shared_libs_kb: u64,
    thread_stacks_kb: u64,
}

fn parse_kb_field(line: &str) -> u64 {
    // Format: "FieldName:    1234 kB"
    line.split_whitespace()
        .nth(1)
        .and_then(|v| v.parse().ok())
        .unwrap_or(0)
}

fn parse_map_region_size(line: &str) -> u64 {
    // Format: "7f1234-7f5678 ..."
    let range = match line.split_whitespace().next() {
        Some(r) => r,
        None => return 0,
    };
    let mut parts = range.split('-');
    let start = parts
        .next()
        .and_then(|s| u64::from_str_radix(s, 16).ok())
        .unwrap_or(0);
    let end = parts
        .next()
        .and_then(|s| u64::from_str_radix(s, 16).ok())
        .unwrap_or(0);
    (end - start) / 1024
}

async fn mem_info() -> Response {
    // Read /proc/self/status
    let status = match fs::read_to_string("/proc/self/status").await {
        Ok(s) => s,
        Err(e) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                format!("failed to read /proc/self/status: {e}"),
            )
                .into_response();
        }
    };

    let mut info = MemInfo {
        rss_kb: 0,
        rss_anon_kb: 0,
        rss_file_kb: 0,
        vm_size_kb: 0,
        vm_peak_kb: 0,
        vm_data_kb: 0,
        vm_stk_kb: 0,
        threads: 0,
        smaps: SmapsRollup::default(),
        maps_summary: MapsSummary::default(),
    };

    for line in status.lines() {
        if line.starts_with("VmRSS:") {
            info.rss_kb = parse_kb_field(line);
        } else if line.starts_with("RssAnon:") {
            info.rss_anon_kb = parse_kb_field(line);
        } else if line.starts_with("RssFile:") {
            info.rss_file_kb = parse_kb_field(line);
        } else if line.starts_with("VmSize:") {
            info.vm_size_kb = parse_kb_field(line);
        } else if line.starts_with("VmPeak:") {
            info.vm_peak_kb = parse_kb_field(line);
        } else if line.starts_with("VmData:") {
            info.vm_data_kb = parse_kb_field(line);
        } else if line.starts_with("VmStk:") {
            info.vm_stk_kb = parse_kb_field(line);
        } else if line.starts_with("Threads:") {
            info.threads = parse_kb_field(line); // reuse parser, no "kB" suffix but works
        }
    }

    // Read /proc/self/smaps_rollup
    if let Ok(smaps) = fs::read_to_string("/proc/self/smaps_rollup").await {
        for line in smaps.lines() {
            if line.starts_with("Pss:") {
                info.smaps.pss_kb = parse_kb_field(line);
            } else if line.starts_with("Pss_Anon:") {
                info.smaps.pss_anon_kb = parse_kb_field(line);
            } else if line.starts_with("Pss_File:") {
                info.smaps.pss_file_kb = parse_kb_field(line);
            } else if line.starts_with("Shared_Clean:") {
                info.smaps.shared_clean_kb = parse_kb_field(line);
            } else if line.starts_with("Private_Dirty:") {
                info.smaps.private_dirty_kb = parse_kb_field(line);
            } else if line.starts_with("Swap:") {
                info.smaps.swap_kb = parse_kb_field(line);
            } else if line.starts_with("AnonHugePages:") {
                info.smaps.anon_huge_pages_kb = parse_kb_field(line);
            }
        }
    }

    // Read /proc/self/maps and summarize
    if let Ok(maps) = fs::read_to_string("/proc/self/maps").await {
        let mut summary = MapsSummary::default();
        for line in maps.lines() {
            summary.total_regions += 1;
            let size_kb = parse_map_region_size(line);
            if line.contains("perf_event") {
                summary.perf_event_rings += 1;
                summary.perf_event_kb += size_kb;
            } else if line.contains("bpf-ringbuf") {
                summary.ring_buf_regions += 1;
                summary.ring_buf_kb += size_kb;
            } else if line.contains("bpf-map") {
                summary.bpf_map_regions += 1;
                summary.bpf_map_kb += size_kb;
            } else if line.contains("[heap]") {
                summary.heap_kb += size_kb;
            } else if line.contains("[stack") {
                summary.thread_stacks_kb += size_kb;
            } else if line.contains("retina-agent") {
                summary.binary_kb += size_kb;
            } else if line.contains(".so") {
                summary.shared_libs_kb += size_kb;
            } else {
                summary.anonymous_kb += size_kb;
            }
        }
        info.maps_summary = summary;
    }

    axum::Json(info).into_response()
}

async fn metrics_handler(State(state): State<DebugState>) -> Response {
    let mut buf = String::new();
    if let Err(e) = encode(&mut buf, &state.metrics.registry) {
        return (
            StatusCode::INTERNAL_SERVER_ERROR,
            format!("encode error: {e}"),
        )
            .into_response();
    }
    let mut response = Response::new(axum::body::Body::from(buf));
    response.headers_mut().insert(
        axum::http::header::CONTENT_TYPE,
        HeaderValue::from_static("application/openmetrics-text; version=1.0.0; charset=utf-8"),
    );
    response
}

async fn healthz(State(state): State<DebugState>) -> Response {
    // Liveness: at least one event reader task is alive.
    if state.state.perf_readers_alive() > 0 {
        (StatusCode::OK, "ok").into_response()
    } else {
        (StatusCode::SERVICE_UNAVAILABLE, "no event readers alive").into_response()
    }
}

async fn readyz(State(state): State<DebugState>) -> Response {
    let mut reasons = Vec::new();

    if !state.state.is_plugin_started() {
        reasons.push("plugin not started");
    }
    if !state.state.is_grpc_bound() {
        reasons.push("gRPC not bound");
    }
    if state.state.perf_readers_alive() == 0 {
        reasons.push("no event readers alive");
    }
    // IpCache: only check if operator is configured (otherwise enrichment is disabled).
    if state.config.operator_addr.is_some() && !state.ip_cache.is_synced() {
        reasons.push("ip cache not synced");
    }

    if reasons.is_empty() {
        (StatusCode::OK, "ok").into_response()
    } else {
        (StatusCode::SERVICE_UNAVAILABLE, reasons.join(", ")).into_response()
    }
}

async fn ipcache_dump(State(state): State<DebugState>) -> impl IntoResponse {
    let entries = state.ip_cache.dump();
    let map: std::collections::BTreeMap<String, serde_json::Value> = entries
        .into_iter()
        .map(|(ip, id)| {
            let mut obj = serde_json::Map::new();
            if !id.namespace.is_empty() {
                obj.insert("namespace".into(), id.namespace.to_string().into());
            }
            if !id.pod_name.is_empty() {
                obj.insert("pod_name".into(), id.pod_name.to_string().into());
            }
            if !id.service_name.is_empty() {
                obj.insert("service_name".into(), id.service_name.to_string().into());
            }
            if !id.node_name.is_empty() {
                obj.insert("node_name".into(), id.node_name.to_string().into());
            }
            if !id.labels.is_empty() {
                let labels: Vec<String> = id.labels.iter().map(|l| l.to_string()).collect();
                obj.insert("labels".into(), labels.into());
            }
            (ip.to_string(), serde_json::Value::Object(obj))
        })
        .collect();
    axum::Json(map)
}

async fn config_dump(State(state): State<DebugState>) -> impl IntoResponse {
    let dump = ConfigDump {
        config: state.config,
        ip_cache: IpCacheStats {
            entries: state.ip_cache.len(),
            synced: state.ip_cache.is_synced(),
        },
        flow_store: FlowStoreStats {
            buffered_flows: state.flow_store.num_flows(),
            total_flows_seen: state.flow_store.seen_flows(),
            capacity: state.flow_store.capacity(),
            uptime_secs: state.flow_store.uptime_ns() as f64 / 1_000_000_000.0,
        },
    };
    axum::Json(dump)
}

async fn pprof_profile(Query(params): Query<ProfileParams>) -> Response {
    let seconds = params.seconds.unwrap_or(30);
    let frequency = params.frequency.unwrap_or(1000);

    let guard = match pprof::ProfilerGuardBuilder::default()
        .frequency(frequency)
        .build()
    {
        Ok(g) => g,
        Err(e) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                format!("failed to start profiler: {e}"),
            )
                .into_response();
        }
    };

    tokio::time::sleep(std::time::Duration::from_secs(seconds)).await;

    let report = match guard.report().build() {
        Ok(r) => r,
        Err(e) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                format!("failed to build report: {e}"),
            )
                .into_response();
        }
    };

    let profile = match report.pprof() {
        Ok(p) => p,
        Err(e) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                format!("failed to generate pprof: {e}"),
            )
                .into_response();
        }
    };

    let mut body = Vec::new();
    if let Err(e) = profile.encode(&mut body) {
        return (
            StatusCode::INTERNAL_SERVER_ERROR,
            format!("failed to encode pprof: {e}"),
        )
            .into_response();
    }

    let mut response = Response::new(axum::body::Body::from(body));
    response.headers_mut().insert(
        axum::http::header::CONTENT_TYPE,
        HeaderValue::from_static("application/octet-stream"),
    );
    response
}

pub async fn serve(
    port: u16,
    config: AgentConfig,
    ip_cache: Arc<IpCache>,
    flow_store: Arc<FlowStore>,
    metrics: Arc<Metrics>,
    agent_state: Arc<AgentState>,
) {
    let state = DebugState {
        config,
        ip_cache,
        flow_store,
        metrics,
        state: agent_state,
    };

    let app = Router::new()
        .route("/metrics", get(metrics_handler))
        .route("/healthz", get(healthz))
        .route("/readyz", get(readyz))
        .route("/debug/config", get(config_dump))
        .route("/debug/ipcache", get(ipcache_dump))
        .route("/debug/mem", get(mem_info))
        .route("/debug/pprof/profile", get(pprof_profile))
        .with_state(state);

    let addr: std::net::SocketAddr = ([0, 0, 0, 0], port).into();
    info!(%addr, "HTTP server listening (metrics, probes, debug)");

    let listener = match tokio::net::TcpListener::bind(addr).await {
        Ok(l) => l,
        Err(e) => {
            tracing::error!("failed to bind metrics port {port}: {e}");
            return;
        }
    };

    if let Err(e) = axum::serve(listener, app).await {
        tracing::error!("debug HTTP server error: {e}");
    }
}
