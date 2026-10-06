mod debug;
mod grpc;
mod state;
mod watchers;

use std::sync::Arc;

use anyhow::Context as _;
use clap::Parser;
use kube::Client;
use tracing::info;

use state::OperatorState;

const GIT_VERSION: &str = env!("GIT_VERSION");
const GIT_COMMIT: &str = env!("GIT_COMMIT");
const RUSTC_VERSION: &str = env!("RUSTC_VERSION");

const UPDATE_BROADCAST_CAPACITY: usize = 8192;

#[derive(Parser)]
#[command(
    name = "retina-operator",
    about = "Retina operator — streams K8s IP-to-identity mappings to agents",
    version = concat!(env!("GIT_VERSION"), " (", env!("GIT_COMMIT"), ", ", env!("RUSTC_VERSION"), ")"),
)]
struct Cli {
    /// gRPC port for `IpCache` service.
    #[arg(long, default_value_t = 9090)]
    grpc_port: u16,

    /// Debug HTTP port.
    #[arg(long, default_value_t = 9091)]
    debug_port: u16,

    /// Log level.
    #[arg(long, default_value = "info")]
    log_level: String,
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
        grpc_port = cli.grpc_port,
        "starting retina-operator",
    );

    let client = Client::try_default()
        .await
        .context("failed to create Kubernetes API client")?;
    let state = Arc::new(OperatorState::new(UPDATE_BROADCAST_CAPACITY));

    // Shutdown signal — notifies the gRPC server to drain gracefully.
    let shutdown = Arc::new(tokio::sync::Notify::new());

    // Spawn K8s watchers.
    let pod_handle = tokio::spawn(watchers::watch_pods(client.clone(), Arc::clone(&state)));
    let svc_handle = tokio::spawn(watchers::watch_services(client.clone(), Arc::clone(&state)));
    let node_handle = tokio::spawn(watchers::watch_nodes(client.clone(), Arc::clone(&state)));

    // Start gRPC server with graceful shutdown support.
    let grpc_handle = {
        let shutdown = Arc::clone(&shutdown);
        tokio::spawn(grpc::serve(cli.grpc_port, Arc::clone(&state), async move {
            shutdown.notified().await;
        }))
    };

    // Start debug HTTP server.
    let debug_handle = tokio::spawn(debug::serve(cli.debug_port, Arc::clone(&state)));

    info!("retina-operator running");

    let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    tokio::select! {
        _ = tokio::signal::ctrl_c() => {},
        _ = sigterm.recv() => {},
    }
    info!("shutting down...");

    // Notify agents of graceful shutdown so they preserve their cache.
    state.broadcast_shutdown();

    // Brief pause for the SHUTDOWN message to propagate through the
    // broadcast channel and MPSC channels to connected agents.
    tokio::time::sleep(std::time::Duration::from_millis(100)).await;

    // Signal the gRPC server to drain in-flight streams gracefully,
    // so agents see a clean end-of-stream instead of an h2 crash.
    shutdown.notify_one();

    // Give streams a moment to drain before aborting everything.
    tokio::time::sleep(std::time::Duration::from_secs(2)).await;

    pod_handle.abort();
    svc_handle.abort();
    node_handle.abort();
    grpc_handle.abort();
    debug_handle.abort();

    info!("retina-operator stopped");
    Ok(())
}
