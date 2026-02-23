use std::sync::Arc;

use retina_proto::ipcache::ip_cache_server::{IpCache, IpCacheServer};
use retina_proto::ipcache::{IpCacheRequest, IpCacheUpdate, ip_cache_update::UpdateType};
use tokio::sync::mpsc;
use tokio_stream::wrappers::ReceiverStream;
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
    }
}

#[tonic::async_trait]
impl IpCache for IpCacheService {
    type StreamUpdatesStream = ReceiverStream<Result<IpCacheUpdate, Status>>;

    async fn stream_updates(
        &self,
        request: Request<IpCacheRequest>,
    ) -> Result<Response<Self::StreamUpdatesStream>, Status> {
        let node_name = request.into_inner().node_name;
        info!(%node_name, "agent connected, starting IP cache stream");

        let (tx, rx) = mpsc::channel(SNAPSHOT_CHANNEL_CAPACITY);

        // Subscribe BEFORE snapshot to avoid missing updates.
        let mut broadcast_rx = self.state.subscribe();

        // Send full snapshot.
        let snapshot = self.state.snapshot();
        let snapshot_len = snapshot.len();

        let tx_clone = tx.clone();
        tokio::spawn(async move {
            // Send all snapshot entries.
            for update in snapshot {
                if tx_clone.send(Ok(update)).await.is_err() {
                    return; // Client disconnected.
                }
            }

            // Send SYNC_COMPLETE marker.
            let sync_complete = IpCacheUpdate {
                update_type: UpdateType::SyncComplete.into(),
                ..Default::default()
            };
            if tx_clone.send(Ok(sync_complete)).await.is_err() {
                return;
            }

            info!(
                node_name,
                snapshot_len, "snapshot sent, streaming incremental updates"
            );

            // Forward incremental updates.
            loop {
                match broadcast_rx.recv().await {
                    Ok(update) => {
                        if tx_clone.send(Ok(update)).await.is_err() {
                            break; // Client disconnected.
                        }
                    }
                    Err(tokio::sync::broadcast::error::RecvError::Lagged(n)) => {
                        warn!(
                            node_name,
                            lagged = n,
                            "broadcast overflow, forcing client reconnect"
                        );
                        let _ = tx_clone
                            .send(Err(Status::data_loss(
                                "broadcast overflow, please reconnect",
                            )))
                            .await;
                        break;
                    }
                    Err(tokio::sync::broadcast::error::RecvError::Closed) => break,
                }
            }
        });

        Ok(Response::new(ReceiverStream::new(rx)))
    }
}

/// Start the IpCache gRPC server with graceful shutdown support.
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
        .await?;

    Ok(())
}
