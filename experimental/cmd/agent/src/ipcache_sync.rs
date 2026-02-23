use std::net::IpAddr;
use std::sync::Arc;
use std::time::Duration;

use retina_core::ipcache::{Identity, IpCache, Workload};
use retina_core::retry::retry_with_backoff;
use retina_core::store::AgentEventStore;
use retina_proto::flow::{AgentEvent, AgentEventType, IpCacheNotification};
use retina_proto::ipcache::ip_cache_client::IpCacheClient;
use retina_proto::ipcache::{IpCacheRequest, ip_cache_update::UpdateType};
use tokio::sync::broadcast;
use tonic::transport::Endpoint;
use tracing::{debug, info, warn};

/// Connect to the operator and stream IP cache updates into the local cache.
///
/// On disconnect, clears the cache and reconnects with exponential backoff.
pub async fn run_ipcache_sync(
    operator_addr: String,
    cache: Arc<IpCache>,
    node_name: String,
    agent_event_tx: broadcast::Sender<Arc<AgentEvent>>,
    agent_event_store: Arc<AgentEventStore>,
) {
    retry_with_backoff("ipcache sync", || {
        let result = try_stream(
            &operator_addr,
            &cache,
            &node_name,
            &agent_event_tx,
            &agent_event_store,
        );
        // Clear cache on disconnect — no stale data.
        let cache = cache.clone();
        async move {
            let r = result.await;
            cache.clear();
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
) -> anyhow::Result<()> {
    let channel = Endpoint::from_shared(operator_addr.to_string())?
        .connect_timeout(Duration::from_secs(5))
        .http2_keep_alive_interval(Duration::from_secs(10))
        .keep_alive_timeout(Duration::from_secs(20))
        .keep_alive_while_idle(true)
        .connect()
        .await?;
    let mut client = IpCacheClient::new(channel);
    info!("connected to operator, requesting stream");

    let request = IpCacheRequest {
        node_name: node_name.to_string(),
    };

    let mut stream = client.stream_updates(request).await?.into_inner();

    while let Some(update) = stream.message().await? {
        let update_type = UpdateType::try_from(update.update_type).unwrap_or(UpdateType::Upsert);

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
            UpdateType::SyncComplete => {
                cache.mark_synced();
                info!("initial IP cache sync complete");
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
