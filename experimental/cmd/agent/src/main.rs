mod debug;
mod grpc;
mod ipcache_sync;

use std::collections::HashSet;
use std::net::{SocketAddr, TcpListener};
use std::sync::Arc;

const GIT_VERSION: &str = env!("GIT_VERSION");
const GIT_COMMIT: &str = env!("GIT_COMMIT");
const RUSTC_VERSION: &str = env!("RUSTC_VERSION");

const FLOW_BROADCAST_CAPACITY: usize = 4096;
const FLOW_STORE_CAPACITY: usize = 4096;
const AGENT_EVENT_BROADCAST_CAPACITY: usize = 256;
const AGENT_EVENT_STORE_CAPACITY: usize = 1024;

use anyhow::Context as _;
use clap::Parser;
use dropreason::plugin::DropReasonPlugin;
use packetparser::plugin::PacketParser;
use retina_core::ipcache::IpCache;
use retina_core::metrics::{AgentState, Metrics};
use retina_core::plugin::{Plugin, PluginContext};
use retina_core::store::AgentEventStore;
use retina_core::store::FlowStore;
use retina_proto::flow::{AgentEvent, AgentEventType, TimeNotification};
use serde::{Deserialize, Serialize};
use tokio::sync::broadcast;
use tracing::{info, warn};

#[derive(Parser)]
#[command(
    name = "retina-agent",
    about = "Retina Rust agent with Hubble gRPC observer",
    version = concat!(env!("GIT_VERSION"), " (", env!("GIT_COMMIT"), ", ", env!("RUSTC_VERSION"), ")"),
)]
struct Cli {
    /// Additional host interfaces to attach TC programs to, beyond the
    /// auto-discovered pod veths. May be specified multiple times.
    /// If omitted, only pod-level veth programs are attached.
    #[arg(long)]
    extra_interfaces: Vec<String>,

    /// Enable pod-level monitoring via veth endpoint programs.
    #[arg(long)]
    pod_level: bool,

    /// Port for Hubble Observer and Peer gRPC services.
    #[arg(long, default_value_t = 4244)]
    hubble_port: u16,

    /// Operator gRPC address for IP cache enrichment (e.g. <http://retina-operator:9090>).
    /// If not set, flow enrichment is disabled.
    #[arg(long)]
    operator_addr: Option<String>,

    /// Packet sampling rate (1 = no sampling, N = report ~1/N packets).
    /// Control-plane events (SYN, FIN, RST) and periodic reports are
    /// always emitted regardless of sampling.
    #[arg(long, default_value_t = 1)]
    sampling_rate: u32,

    /// BPF ring buffer size in bytes (must be a power of 2, minimum 65536).
    /// Only used on kernels >= 5.8; ignored when falling back to perf buffers.
    /// Default: 2MB. Increase for high-throughput nodes to reduce drops.
    #[arg(long, default_value_t = 2_097_152)]
    ring_buffer_size: u32,

    /// Enable the dropreason plugin for packet drop monitoring.
    /// Requires kernel >= 5.5 (x86) / 6.0 (arm64) with BTF support.
    #[arg(long)]
    enable_dropreason: bool,

    /// BPF ring buffer size for dropreason events in bytes (power of 2).
    /// Only used on kernels >= 5.8; ignored when falling back to perf buffers.
    /// Default: 1MB.
    #[arg(long, default_value_t = 1_048_576)]
    dropreason_ring_buffer_size: u32,

    /// Path to dropreason filter config (YAML with suppressedDropReasons list).
    /// Mounted from `ConfigMap`. If the file is missing, no filtering is applied.
    #[arg(long, default_value = "/etc/retina/dropreason-filter.yaml")]
    dropreason_filter_path: String,

    /// Log level.
    #[arg(long, default_value = "info")]
    log_level: String,

    /// HTTP port for metrics, health probes, and debug endpoints.
    #[arg(long, default_value_t = 10093)]
    metrics_port: u16,
}

#[derive(Clone, Serialize)]
pub struct AgentConfig {
    pub extra_interfaces: Vec<String>,
    pub pod_level: bool,
    pub hubble_port: u16,
    pub operator_addr: Option<String>,
    pub sampling_rate: u32,
    pub ring_buffer_size: u32,
    pub enable_dropreason: bool,
    pub dropreason_ring_buffer_size: u32,
    pub suppressed_drop_reasons: Vec<String>,
    pub log_level: String,
    pub metrics_port: u16,
}

#[derive(Deserialize, Default)]
#[serde(rename_all = "camelCase")]
struct DropReasonFilterConfig {
    #[serde(default)]
    suppressed_drop_reasons: Vec<String>,
}

fn load_dropreason_filter(path: &str) -> HashSet<String> {
    let content = match std::fs::read_to_string(path) {
        Ok(c) => c,
        Err(e) => {
            if e.kind() != std::io::ErrorKind::NotFound {
                warn!("dropreason: failed to read filter config {path}: {e}");
            }
            return HashSet::new();
        }
    };
    match serde_yaml::from_str::<DropReasonFilterConfig>(&content) {
        Ok(config) => config.suppressed_drop_reasons.into_iter().collect(),
        Err(e) => {
            warn!("dropreason: failed to parse filter config {path}: {e}");
            HashSet::new()
        }
    }
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_new(&cli.log_level)
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .init();

    info!(
        version = GIT_VERSION,
        commit = GIT_COMMIT,
        rustc = RUSTC_VERSION,
        extra_interfaces = ?cli.extra_interfaces,
        hubble_port = cli.hubble_port,
        pod_level = cli.pod_level,
        sampling_rate = cli.sampling_rate,
        "starting retina-agent",
    );

    // Pre-flight: verify the gRPC port is available. A stale retina-agent
    // process holding this port is a common dev pitfall — fail fast with a
    // clear message instead of silently losing traffic to the old process.
    let grpc_addr: SocketAddr = format!("0.0.0.0:{}", cli.hubble_port).parse()?;
    if let Err(e) = TcpListener::bind(grpc_addr) {
        anyhow::bail!(
            "cannot bind gRPC port {}: {}\n\
             Hint: a stale retina-agent may already be running. Try:\n  \
             sudo lsof -i :{} -P -n\n  \
             sudo kill <PID>",
            cli.hubble_port,
            e,
            cli.hubble_port,
        );
    }
    // Drop the test listener immediately so tonic can bind it.

    // Broadcast channel for flow fan-out to gRPC subscribers.
    let (flow_tx, _) = broadcast::channel::<Arc<retina_proto::flow::Flow>>(FLOW_BROADCAST_CAPACITY);

    // Flow ring buffer for historical queries.
    let flow_store = Arc::new(FlowStore::new(FLOW_STORE_CAPACITY));

    // Broadcast channel and store for agent events.
    let (agent_event_tx, _) = broadcast::channel::<Arc<AgentEvent>>(AGENT_EVENT_BROADCAST_CAPACITY);
    let agent_event_store = Arc::new(AgentEventStore::new(AGENT_EVENT_STORE_CAPACITY));

    // IP cache for flow enrichment.
    let ip_cache = Arc::new(IpCache::new());

    // Set local node name so the enricher can distinguish Host vs RemoteNode.
    let local_node_name = hostname::get()
        .map(|h| h.to_string_lossy().into_owned())
        .unwrap_or_default();
    ip_cache.set_local_node_name(local_node_name.clone());

    // Emit AGENT_STARTED event.
    {
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default();
        let started_event = Arc::new(AgentEvent {
            r#type: AgentEventType::AgentStarted.into(),
            notification: Some(retina_proto::flow::agent_event::Notification::AgentStart(
                TimeNotification {
                    time: Some(prost_types::Timestamp {
                        seconds: now.as_secs() as i64,
                        nanos: now.subsec_nanos() as i32,
                    }),
                },
            )),
        });
        agent_event_store.push(Arc::clone(&started_event));
        let _ = agent_event_tx.send(started_event);
    }

    // Metrics and agent state for observability and health probes.
    let metrics = Arc::new(Metrics::new());
    let agent_state = Arc::new(AgentState::new());

    // Build plugin context and start the packetparser plugin.
    let ctx = PluginContext {
        flow_tx: flow_tx.clone(),
        flow_store: Arc::clone(&flow_store),
        ip_cache: Arc::clone(&ip_cache),
        metrics: Arc::clone(&metrics),
        state: Arc::clone(&agent_state),
    };

    let mut plugin = PacketParser::new(
        cli.extra_interfaces.clone(),
        cli.pod_level,
        cli.sampling_rate,
        cli.ring_buffer_size,
    );
    plugin
        .start(ctx)
        .await
        .context("failed to start packetparser plugin")?;

    // Load dropreason filter once (used by both the plugin and the debug config).
    let suppressed_reasons = load_dropreason_filter(&cli.dropreason_filter_path);

    // Optionally start the dropreason plugin.
    let mut dropreason_plugin: Option<DropReasonPlugin> = if cli.enable_dropreason {
        let dr_ctx = PluginContext {
            flow_tx: flow_tx.clone(),
            flow_store: Arc::clone(&flow_store),
            ip_cache: Arc::clone(&ip_cache),
            metrics: Arc::clone(&metrics),
            state: Arc::clone(&agent_state),
        };
        if !suppressed_reasons.is_empty() {
            info!(count = suppressed_reasons.len(), reasons = ?suppressed_reasons, "dropreason: loaded suppress filter");
        }
        let mut dr =
            DropReasonPlugin::new(cli.dropreason_ring_buffer_size, suppressed_reasons.clone());
        dr.start(dr_ctx)
            .await
            .context("failed to start dropreason plugin")?;
        Some(dr)
    } else {
        None
    };

    // Spawn IP cache sync if operator address is provided.
    let ipcache_handle = if let Some(ref addr) = cli.operator_addr {
        let cache = Arc::clone(&ip_cache);
        let addr = addr.clone();
        let node_name = local_node_name.clone();
        let event_tx = agent_event_tx.clone();
        let event_store = Arc::clone(&agent_event_store);
        Some(tokio::spawn(async move {
            ipcache_sync::run_ipcache_sync(addr, cache, node_name, event_tx, event_store).await;
        }))
    } else {
        info!("no --operator-addr set, flow enrichment disabled");
        None
    };

    // Start HTTP server (metrics, probes, debug).
    let agent_config = AgentConfig {
        extra_interfaces: cli.extra_interfaces.clone(),
        pod_level: cli.pod_level,
        hubble_port: cli.hubble_port,
        operator_addr: cli.operator_addr.clone(),
        sampling_rate: cli.sampling_rate,
        ring_buffer_size: cli.ring_buffer_size,
        enable_dropreason: cli.enable_dropreason,
        dropreason_ring_buffer_size: cli.dropreason_ring_buffer_size,
        suppressed_drop_reasons: suppressed_reasons.iter().cloned().collect(),
        log_level: cli.log_level.clone(),
        metrics_port: cli.metrics_port,
    };
    let debug_handle = tokio::spawn(debug::serve(
        cli.metrics_port,
        agent_config,
        Arc::clone(&ip_cache),
        Arc::clone(&flow_store),
        Arc::clone(&metrics),
        Arc::clone(&agent_state),
    ));

    // Start gRPC server.
    let grpc_handle = tokio::spawn(grpc::serve(
        cli.hubble_port,
        local_node_name,
        flow_tx,
        flow_store,
        agent_event_tx,
        agent_event_store,
        agent_state,
        ip_cache,
    ));

    info!("retina-agent running");

    // Wait for shutdown signal (SIGINT or SIGTERM).
    let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    tokio::select! {
        _ = tokio::signal::ctrl_c() => {},
        _ = sigterm.recv() => {},
    }

    info!("shutting down...");

    // Stop plugins.
    if let Some(ref mut dr) = dropreason_plugin {
        dr.stop().await?;
    }
    plugin.stop().await?;

    // Abort agent-level tasks.
    grpc_handle.abort();
    debug_handle.abort();
    if let Some(h) = ipcache_handle {
        h.abort();
    }

    info!("retina-agent stopped");
    Ok(())
}
