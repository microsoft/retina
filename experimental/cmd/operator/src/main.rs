mod grpc;
mod state;
mod watchers;

use std::sync::Arc;

use clap::Parser;
use kube::Client;
use tracing::info;

use state::OperatorState;

#[derive(Parser)]
#[command(
    name = "retina-operator",
    about = "Retina operator — streams K8s IP-to-identity mappings to agents"
)]
struct Cli {
    /// gRPC port for IpCache service.
    #[arg(long, default_value_t = 9090)]
    grpc_port: u16,

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

    info!(grpc_port = cli.grpc_port, "starting retina-operator");

    let client = Client::try_default().await?;
    let state = Arc::new(OperatorState::new(8192));

    // Spawn K8s watchers.
    let pod_handle = tokio::spawn(watchers::watch_pods(client.clone(), state.clone()));
    let svc_handle = tokio::spawn(watchers::watch_services(client.clone(), state.clone()));
    let node_handle = tokio::spawn(watchers::watch_nodes(client.clone(), state.clone()));

    // Start gRPC server.
    let grpc_handle = tokio::spawn(grpc::serve(cli.grpc_port, state));

    info!("retina-operator running");

    let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    tokio::select! {
        _ = tokio::signal::ctrl_c() => {},
        _ = sigterm.recv() => {},
    }
    info!("shutting down...");

    pod_handle.abort();
    svc_handle.abort();
    node_handle.abort();
    grpc_handle.abort();

    info!("retina-operator stopped");
    Ok(())
}
