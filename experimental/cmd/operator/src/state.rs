use std::collections::HashMap;
use std::net::IpAddr;
use std::sync::RwLock;

use retina_proto::ipcache::{ip_cache_update::UpdateType, IpCacheUpdate, Workload};
use tokio::sync::broadcast;

/// Canonical operator state: maps IPs to identities and broadcasts changes.
pub struct OperatorState {
    cache: RwLock<HashMap<IpAddr, CachedIdentity>>,
    updates_tx: broadcast::Sender<IpCacheUpdate>,
}

#[derive(Clone)]
pub struct CachedIdentity {
    pub namespace: String,
    pub pod_name: String,
    pub service_name: String,
    pub node_name: String,
    pub labels: Vec<String>,
    pub workloads: Vec<Workload>,
}

impl OperatorState {
    pub fn new(broadcast_capacity: usize) -> Self {
        let (updates_tx, _) = broadcast::channel(broadcast_capacity);
        Self {
            cache: RwLock::new(HashMap::new()),
            updates_tx,
        }
    }

    pub fn upsert(&self, ip: IpAddr, identity: CachedIdentity) {
        let update = IpCacheUpdate {
            update_type: UpdateType::Upsert.into(),
            ip: ip.to_string(),
            namespace: identity.namespace.clone(),
            pod_name: identity.pod_name.clone(),
            service_name: identity.service_name.clone(),
            node_name: identity.node_name.clone(),
            labels: identity.labels.clone(),
            workloads: identity.workloads.clone(),
        };
        self.cache.write().unwrap().insert(ip, identity);
        // Ignore error if no subscribers.
        let _ = self.updates_tx.send(update);
    }

    pub fn delete(&self, ip: &IpAddr) {
        if self.cache.write().unwrap().remove(ip).is_some() {
            let update = IpCacheUpdate {
                update_type: UpdateType::Delete.into(),
                ip: ip.to_string(),
                ..Default::default()
            };
            let _ = self.updates_tx.send(update);
        }
    }

    /// Take a consistent snapshot of all entries as UPSERT messages.
    pub fn snapshot(&self) -> Vec<IpCacheUpdate> {
        let cache = self.cache.read().unwrap();
        cache
            .iter()
            .map(|(ip, id)| IpCacheUpdate {
                update_type: UpdateType::Upsert.into(),
                ip: ip.to_string(),
                namespace: id.namespace.clone(),
                pod_name: id.pod_name.clone(),
                service_name: id.service_name.clone(),
                node_name: id.node_name.clone(),
                labels: id.labels.clone(),
                workloads: id.workloads.clone(),
            })
            .collect()
    }

    /// Subscribe to incremental updates. Must be called BEFORE snapshot()
    /// to avoid missing updates between snapshot and subscribe.
    pub fn subscribe(&self) -> broadcast::Receiver<IpCacheUpdate> {
        self.updates_tx.subscribe()
    }

    /// Return the number of entries in the cache.
    pub fn len(&self) -> usize {
        self.cache.read().unwrap().len()
    }

    /// Return a snapshot of all entries for debugging.
    pub fn dump(&self) -> Vec<(IpAddr, CachedIdentity)> {
        self.cache
            .read()
            .unwrap()
            .iter()
            .map(|(ip, id)| (*ip, id.clone()))
            .collect()
    }
}
