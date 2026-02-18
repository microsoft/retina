use std::net::IpAddr;
use std::sync::Arc;
use std::time::Duration;

use retina_core::ipcache::{Identity, IpCache, Workload};
use retina_proto::ipcache::ip_cache_client::IpCacheClient;
use retina_proto::ipcache::{ip_cache_update::UpdateType, IpCacheRequest};
use tracing::{debug, error, info, warn};

/// Connect to the operator and stream IP cache updates into the local cache.
///
/// On disconnect, clears the cache and reconnects with exponential backoff.
pub async fn run_ipcache_sync(operator_addr: String, cache: Arc<IpCache>, node_name: String) {
    let mut backoff = Duration::from_secs(1);
    let max_backoff = Duration::from_secs(60);

    loop {
        info!(%operator_addr, "connecting to retina-operator");

        match try_stream(&operator_addr, &cache, &node_name).await {
            Ok(()) => {
                info!("operator stream ended cleanly");
            }
            Err(e) => {
                error!("operator stream error: {}", e);
            }
        }

        // Clear cache on disconnect — no stale data.
        cache.clear();
        info!(backoff_secs = backoff.as_secs(), "reconnecting after backoff");
        tokio::time::sleep(backoff).await;
        backoff = (backoff * 2).min(max_backoff);
    }
}

async fn try_stream(
    operator_addr: &str,
    cache: &IpCache,
    node_name: &str,
) -> anyhow::Result<()> {
    let mut client = IpCacheClient::connect(operator_addr.to_string()).await?;
    info!("connected to operator, requesting stream");

    // Reset backoff on successful connection (caller manages backoff variable,
    // but a successful stream implicitly resets it since we return Ok).
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
                    namespace: update.namespace,
                    pod_name: update.pod_name,
                    service_name: update.service_name,
                    node_name: update.node_name,
                    labels: update.labels,
                    workloads: update
                        .workloads
                        .into_iter()
                        .map(|w| Workload {
                            name: w.name,
                            kind: w.kind,
                        })
                        .collect(),
                };
                debug!(%ip, ns = %identity.namespace, pod = %identity.pod_name, svc = %identity.service_name, node = %identity.node_name, "upsert");
                cache.upsert(ip, identity);
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
            }
            UpdateType::SyncComplete => {
                cache.mark_synced();
                info!("initial IP cache sync complete");
            }
        }
    }

    Ok(())
}
