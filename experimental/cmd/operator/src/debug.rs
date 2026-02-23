use std::sync::Arc;
use std::sync::atomic::Ordering;

use axum::Router;
use axum::extract::State;
use axum::response::IntoResponse;
use axum::routing::get;
use tracing::info;

use crate::state::OperatorState;

#[derive(Clone)]
struct DebugState {
    state: Arc<OperatorState>,
}

async fn ipcache_dump(State(state): State<DebugState>) -> impl IntoResponse {
    let entries = state.state.dump();
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
                obj.insert(
                    "labels".into(),
                    id.labels
                        .iter()
                        .map(|l| l.to_string())
                        .collect::<Vec<_>>()
                        .into(),
                );
            }
            (ip.to_string(), serde_json::Value::Object(obj))
        })
        .collect();
    axum::Json(map)
}

async fn stats(State(state): State<DebugState>) -> impl IntoResponse {
    let dump = state.state.dump();
    let nodes = dump
        .iter()
        .filter(|(_, id)| !id.node_name.is_empty())
        .count();
    let pods = dump
        .iter()
        .filter(|(_, id)| !id.pod_name.is_empty())
        .count();
    let services = dump
        .iter()
        .filter(|(_, id)| !id.service_name.is_empty())
        .count();
    axum::Json(serde_json::json!({
        "total_entries": dump.len(),
        "nodes": nodes,
        "pods": pods,
        "services": services,
        "upserts_total": state.state.upserts_total.load(Ordering::Relaxed),
        "upserts_skipped": state.state.upserts_skipped.load(Ordering::Relaxed),
        "deletes_total": state.state.deletes_total.load(Ordering::Relaxed),
        "connected_agents": state.state.connected_agents.load(Ordering::Relaxed),
        "broadcast_queue_depth": state.state.broadcast_queue_depth(),
        "broadcast_capacity": state.state.broadcast_capacity(),
    }))
}

pub async fn serve(port: u16, state: Arc<OperatorState>) {
    let debug_state = DebugState { state };

    let app = Router::new()
        .route("/debug/ipcache", get(ipcache_dump))
        .route("/debug/stats", get(stats))
        .with_state(debug_state);

    let addr: std::net::SocketAddr = ([0, 0, 0, 0], port).into();
    info!(%addr, "debug HTTP server listening");

    let listener = match tokio::net::TcpListener::bind(addr).await {
        Ok(l) => l,
        Err(e) => {
            tracing::error!("failed to bind debug port {port}: {e}");
            return;
        }
    };

    if let Err(e) = axum::serve(listener, app).await {
        tracing::error!("debug HTTP server error: {e}");
    }
}
