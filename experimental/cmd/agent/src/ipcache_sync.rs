use std::net::IpAddr;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use retina_core::ipcache::{Identity, IpCache, Workload};
use retina_core::retry::retry_with_backoff;
use retina_core::store::AgentEventStore;
use retina_proto::flow::{AgentEvent, AgentEventType, IpCacheNotification};
use retina_proto::ipcache::ip_cache_client::IpCacheClient;
use retina_proto::ipcache::{IpCacheRequest, ip_cache_update::UpdateType};
use tokio::sync::broadcast;
use tonic::codec::CompressionEncoding;
use tonic::transport::Endpoint;
use tracing::{debug, info, warn};

/// Connect to the operator and stream IP cache updates into the local cache.
///
/// On disconnect, clears the cache and reconnects with exponential backoff.
/// On graceful operator shutdown (SHUTDOWN message), preserves the cache so
/// flow enrichment continues uninterrupted during rolling updates.
pub async fn run_ipcache_sync(
    operator_addr: String,
    cache: Arc<IpCache>,
    node_name: String,
    agent_event_tx: broadcast::Sender<Arc<AgentEvent>>,
    agent_event_store: Arc<AgentEventStore>,
) {
    let preserve_cache = Arc::new(AtomicBool::new(false));

    retry_with_backoff("ipcache sync", || {
        // Reset the preserve flag for each connection attempt.
        preserve_cache.store(false, Ordering::Release);

        let result = try_stream(
            &operator_addr,
            &cache,
            &node_name,
            &agent_event_tx,
            &agent_event_store,
            preserve_cache.clone(),
        );
        let cache = cache.clone();
        let preserve = preserve_cache.clone();
        async move {
            let r = result.await;
            // Only clear cache on unexpected disconnects; preserve it
            // when the operator sent a graceful SHUTDOWN message.
            if !preserve.load(Ordering::Acquire) {
                cache.clear();
            }
            r
        }
    })
    .await
}

async fn try_stream(
    operator_addr: &str,
    cache: &IpCache,
    node_name: &str,
    agent_event_tx: &broadcast::Sender<Arc<AgentEvent>>,
    agent_event_store: &AgentEventStore,
    preserve_cache: Arc<AtomicBool>,
) -> anyhow::Result<()> {
    let channel = Endpoint::from_shared(operator_addr.to_string())?
        .connect_timeout(Duration::from_secs(5))
        .http2_keep_alive_interval(Duration::from_secs(10))
        .keep_alive_timeout(Duration::from_secs(20))
        .keep_alive_while_idle(true)
        .connect()
        .await?;
    let mut client = IpCacheClient::new(channel)
        .accept_compressed(CompressionEncoding::Gzip)
        .send_compressed(CompressionEncoding::Gzip);
    info!("connected to operator, requesting stream");

    let request = IpCacheRequest {
        node_name: node_name.to_string(),
    };

    let mut stream = client.stream_updates(request).await?.into_inner();

    let mut initial_sync_done = false;
    let mut snapshot_count: u64 = 0;

    while let Some(batch) = stream.message().await? {
        for update in batch.updates {
            let update_type =
                UpdateType::try_from(update.update_type).unwrap_or(UpdateType::Upsert);

            match update_type {
                UpdateType::Upsert => {
                    let ip: IpAddr = match update.ip.parse() {
                        Ok(ip) => ip,
                        Err(e) => {
                            warn!(ip = %update.ip, "invalid IP in upsert: {}", e);
                            continue;
                        }
                    };
                    let identity = Identity {
                        namespace: Arc::from(update.namespace.as_str()),
                        pod_name: Arc::from(update.pod_name.as_str()),
                        service_name: Arc::from(update.service_name.as_str()),
                        node_name: Arc::from(update.node_name.as_str()),
                        labels: update
                            .labels
                            .iter()
                            .map(|l| Arc::from(l.as_str()))
                            .collect::<Vec<_>>()
                            .into(),
                        workloads: update
                            .workloads
                            .into_iter()
                            .map(|w| Workload {
                                name: Arc::from(w.name.as_str()),
                                kind: Arc::from(w.kind.as_str()),
                            })
                            .collect::<Vec<_>>()
                            .into(),
                    };
                    debug!(%ip, ns = %identity.namespace, pod = %identity.pod_name, svc = %identity.service_name, node = %identity.node_name, "upsert");
                    cache.upsert(ip, identity);

                    // Skip per-entry agent events during initial snapshot to avoid
                    // flooding the agent event broadcast channel (capacity 256).
                    if initial_sync_done {
                        emit_agent_event(
                            AgentEventType::IpcacheUpserted,
                            IpCacheNotification {
                                cidr: update.ip,
                                namespace: update.namespace,
                                pod_name: update.pod_name,
                                ..Default::default()
                            },
                            agent_event_store,
                            agent_event_tx,
                        );
                    } else {
                        snapshot_count += 1;
                    }
                }
                UpdateType::Delete => {
                    let ip: IpAddr = match update.ip.parse() {
                        Ok(ip) => ip,
                        Err(e) => {
                            warn!(ip = %update.ip, "invalid IP in delete: {}", e);
                            continue;
                        }
                    };
                    cache.delete(&ip);

                    if initial_sync_done {
                        emit_agent_event(
                            AgentEventType::IpcacheDeleted,
                            IpCacheNotification {
                                cidr: update.ip,
                                ..Default::default()
                            },
                            agent_event_store,
                            agent_event_tx,
                        );
                    }
                }
                UpdateType::SyncComplete => {
                    cache.mark_synced();
                    initial_sync_done = true;
                    info!(entries = snapshot_count, "initial IP cache sync complete");
                }
                UpdateType::Shutdown => {
                    // Operator is shutting down gracefully (e.g. rolling update).
                    // Preserve cache so enrichment continues while reconnecting.
                    preserve_cache.store(true, Ordering::Release);
                    info!("operator shutting down, preserving cache for reconnect");
                    return Ok(());
                }
            }
        }
    }

    Ok(())
}

fn emit_agent_event(
    event_type: AgentEventType,
    notification: IpCacheNotification,
    store: &AgentEventStore,
    tx: &broadcast::Sender<Arc<AgentEvent>>,
) {
    let event = Arc::new(AgentEvent {
        r#type: event_type.into(),
        notification: Some(
            retina_proto::flow::agent_event::Notification::IpcacheUpdate(notification),
        ),
    });
    store.push(event.clone());
    let _ = tx.send(event);
}
