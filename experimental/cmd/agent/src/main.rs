mod debug;
mod grpc;
mod ipcache_sync;

use std::net::{SocketAddr, TcpListener};
use std::sync::Arc;

use anyhow::Context as _;
use clap::Parser;
use packetparser::plugin::PacketParser;
use retina_core::ipcache::IpCache;
use retina_core::metrics::{AgentState, Metrics};
use retina_core::plugin::{Plugin, PluginContext};
use retina_core::store::FlowStore;
use serde::Serialize;
use tokio::sync::broadcast;
use tracing::info;

#[derive(Parser)]
#[command(
    name = "retina-agent",
    about = "Retina Rust agent with Hubble gRPC observer"
)]
struct Cli {
    /// Network interface to attach host TC programs to.
    /// If omitted, host programs are loaded but not attached (pod-level only).
    #[arg(long)]
    interface: Option<String>,

    /// Enable pod-level monitoring via veth endpoint programs.
    #[arg(long)]
    pod_level: bool,

    /// gRPC port for Hubble Observer service.
    #[arg(long, default_value_t = 4244)]
    grpc_port: u16,

    /// Operator gRPC address for IP cache enrichment (e.g. http://retina-operator:9090).
    /// If not set, flow enrichment is disabled.
    #[arg(long)]
    operator_addr: Option<String>,

    /// Log level.
    #[arg(long, default_value = "info")]
    log_level: String,

    /// HTTP port for metrics, health probes, and debug endpoints.
    #[arg(long, default_value_t = 10093)]
    metrics_port: u16,
}

#[derive(Clone, Serialize)]
pub struct AgentConfig {
    pub interface: Option<String>,
    pub pod_level: bool,
    pub grpc_port: u16,
    pub operator_addr: Option<String>,
    pub log_level: String,
    pub metrics_port: u16,
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

    info!(interface = ?cli.interface, grpc_port = cli.grpc_port, pod_level = cli.pod_level, "starting retina-agent");

    // Pre-flight: verify the gRPC port is available. A stale retina-agent
    // process holding this port is a common dev pitfall — fail fast with a
    // clear message instead of silently losing traffic to the old process.
    let grpc_addr: SocketAddr = format!("0.0.0.0:{}", cli.grpc_port).parse()?;
    if let Err(e) = TcpListener::bind(grpc_addr) {
        anyhow::bail!(
            "cannot bind gRPC port {}: {}\n\
             Hint: a stale retina-agent may already be running. Try:\n  \
             sudo lsof -i :{} -P -n\n  \
             sudo kill <PID>",
            cli.grpc_port,
            e,
            cli.grpc_port,
        );
    }
    // Drop the test listener immediately so tonic can bind it.

    // Broadcast channel for flow fan-out to gRPC subscribers.
    let (flow_tx, _) = broadcast::channel::<Arc<retina_proto::flow::Flow>>(4096);

    // Flow ring buffer for historical queries.
    let flow_store = Arc::new(FlowStore::new(4096));

    // IP cache for flow enrichment.
    let ip_cache = Arc::new(IpCache::new());

    // Set local node name so the enricher can distinguish Host vs RemoteNode.
    let local_node_name = hostname::get()
        .map(|h| h.to_string_lossy().into_owned())
        .unwrap_or_default();
    ip_cache.set_local_node_name(local_node_name.clone());

    // Metrics and agent state for observability and health probes.
    let metrics = Arc::new(Metrics::new());
    let agent_state = Arc::new(AgentState::new());

    // Build plugin context and start the packetparser plugin.
    let ctx = PluginContext {
        flow_tx: flow_tx.clone(),
        flow_store: flow_store.clone(),
        ip_cache: ip_cache.clone(),
        metrics: metrics.clone(),
        state: agent_state.clone(),
    };

    let mut plugin = PacketParser::new(cli.interface.clone(), cli.pod_level);
    plugin
        .start(ctx)
        .await
        .context("failed to start packetparser plugin")?;

    // Spawn IP cache sync if operator address is provided.
    let ipcache_handle = if let Some(ref addr) = cli.operator_addr {
        let cache = ip_cache.clone();
        let addr = addr.clone();
        let node_name = local_node_name;
        Some(tokio::spawn(async move {
            ipcache_sync::run_ipcache_sync(addr, cache, node_name).await;
        }))
    } else {
        info!("no --operator-addr set, flow enrichment disabled");
        None
    };

    // Start HTTP server (metrics, probes, debug).
    let agent_config = AgentConfig {
        interface: cli.interface.clone(),
        pod_level: cli.pod_level,
        grpc_port: cli.grpc_port,
        operator_addr: cli.operator_addr.clone(),
        log_level: cli.log_level.clone(),
        metrics_port: cli.metrics_port,
    };
    let debug_handle = tokio::spawn(debug::serve(
        cli.metrics_port,
        agent_config,
        ip_cache.clone(),
        flow_store.clone(),
        metrics.clone(),
        agent_state.clone(),
    ));

    // Start gRPC server.
    let grpc_handle = tokio::spawn(grpc::serve(cli.grpc_port, flow_tx, flow_store, agent_state));

    info!("retina-agent running");

    // Wait for shutdown signal (SIGINT or SIGTERM).
    let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    tokio::select! {
        _ = tokio::signal::ctrl_c() => {},
        _ = sigterm.recv() => {},
    }

    info!("shutting down...");

    // Stop the plugin (aborts eBPF tasks, drops TC filters).
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
