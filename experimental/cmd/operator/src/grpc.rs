use std::sync::Arc;
use std::sync::atomic::Ordering;

use anyhow::Context as _;
use retina_proto::ipcache::ip_cache_server::{IpCache, IpCacheServer};
use retina_proto::ipcache::{
    IpCacheBatch, IpCacheRequest, IpCacheUpdate, ip_cache_update::UpdateType,
};
use tokio::sync::mpsc;
use tokio_stream::wrappers::ReceiverStream;
use tonic::codec::CompressionEncoding;
use tonic::{Request, Response, Status};
use tracing::{info, warn};

use crate::state::OperatorState;

const SNAPSHOT_CHANNEL_CAPACITY: usize = 512;

pub struct IpCacheService {
    state: Arc<OperatorState>,
}

impl IpCacheService {
    pub fn new(state: Arc<OperatorState>) -> Self {
        Self { state }
    }

    pub fn into_server(self) -> IpCacheServer<Self> {
        IpCacheServer::new(self)
            .accept_compressed(CompressionEncoding::Gzip)
            .send_compressed(CompressionEncoding::Gzip)
    }
}

/// Drop guard that decrements the connected agent counter when the
/// per-agent streaming task ends (by any means, including panics).
struct AgentDisconnectGuard(Arc<OperatorState>);

impl Drop for AgentDisconnectGuard {
    fn drop(&mut self) {
        self.0.connected_agents.fetch_sub(1, Ordering::Relaxed);
    }
}

#[tonic::async_trait]
impl IpCache for IpCacheService {
    type StreamUpdatesStream = ReceiverStream<Result<IpCacheBatch, Status>>;

    async fn stream_updates(
        &self,
        request: Request<IpCacheRequest>,
    ) -> Result<Response<Self::StreamUpdatesStream>, Status> {
        let node_name = request.into_inner().node_name;
        self.state.connected_agents.fetch_add(1, Ordering::Relaxed);
        info!(%node_name, "agent connected, starting IP cache stream");

        let (tx, rx) = mpsc::channel(SNAPSHOT_CHANNEL_CAPACITY);

        // Subscribe BEFORE snapshot to avoid missing updates.
        let mut broadcast_rx = self.state.subscribe();

        // Send full snapshot as a single batch message.
        let snapshot = self.state.snapshot();
        let snapshot_len = snapshot.len();

        let state = Arc::clone(&self.state);
        tokio::spawn(async move {
            // Guard decrements connected_agents on any exit.
            let _guard = AgentDisconnectGuard(state);

            // Send entire snapshot as one batch (single gRPC frame).
            let snapshot_batch = IpCacheBatch { updates: snapshot };
            if tx.send(Ok(snapshot_batch)).await.is_err() {
                return; // Client disconnected.
            }

            // Send SYNC_COMPLETE marker.
            let sync_complete = IpCacheBatch {
                updates: vec![IpCacheUpdate {
                    update_type: UpdateType::SyncComplete.into(),
                    ..Default::default()
                }],
            };
            if tx.send(Ok(sync_complete)).await.is_err() {
                return;
            }

            info!(
                node_name,
                snapshot_len, "snapshot sent, streaming incremental updates"
            );

            // Forward incremental updates.
            // Use select! to also detect client disconnect via tx.closed(),
            // so we don't leak tasks when no broadcast updates are flowing
            // (e.g. when change detection skips redundant upserts).
            loop {
                tokio::select! {
                    result = broadcast_rx.recv() => {
                        match result {
                            Ok(update) => {
                                let batch = IpCacheBatch { updates: vec![update] };
                                if tx.send(Ok(batch)).await.is_err() {
                                    break; // Client disconnected.
                                }
                            }
                            Err(tokio::sync::broadcast::error::RecvError::Lagged(n)) => {
                                warn!(
                                    node_name,
                                    lagged = n,
                                    "broadcast overflow, forcing client reconnect"
                                );
                                let _ = tx
                                    .send(Err(Status::data_loss(
                                        "broadcast overflow, please reconnect",
                                    )))
                                    .await;
                                break;
                            }
                            Err(tokio::sync::broadcast::error::RecvError::Closed) => break,
                        }
                    }
                    () = tx.closed() => {
                        info!(node_name, "agent disconnected");
                        break;
                    }
                }
            }
        });

        Ok(Response::new(ReceiverStream::new(rx)))
    }
}

/// Start the `IpCache` gRPC server with graceful shutdown support.
///
/// When the `shutdown` future completes, the server stops accepting new RPCs
/// and drains in-flight streams so agents see a clean end-of-stream instead
/// of an h2 protocol error.
pub async fn serve(
    port: u16,
    state: Arc<OperatorState>,
    shutdown: impl std::future::Future<Output = ()>,
) -> anyhow::Result<()> {
    let addr = format!("0.0.0.0:{port}").parse()?;
    let service = IpCacheService::new(state);

    info!(%addr, "starting IpCache gRPC server");

    tonic::transport::Server::builder()
        .http2_keepalive_interval(Some(std::time::Duration::from_secs(10)))
        .http2_keepalive_timeout(Some(std::time::Duration::from_secs(20)))
        .add_service(service.into_server())
        .serve_with_shutdown(addr, shutdown)
        .await
        .context("IpCache gRPC server failed")?;

    Ok(())
}
