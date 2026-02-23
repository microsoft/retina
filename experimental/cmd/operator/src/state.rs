use std::collections::HashMap;
use std::net::IpAddr;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, RwLock};

use retina_proto::ipcache::{
    IpCacheUpdate, Workload as ProtoWorkload, ip_cache_update::UpdateType,
};
use tokio::sync::broadcast;

/// Kind of Kubernetes resource that owns an IP cache entry.
#[derive(Clone, Copy, PartialEq, Eq)]
pub enum ResourceKind {
    Pod,
    Service,
    Node,
}

#[derive(Clone, PartialEq, Eq)]
pub struct CachedWorkload {
    pub name: Arc<str>,
    pub kind: Arc<str>,
}

/// Fields use `Arc<str>` so that cloning is nearly free (atomic ref bumps
/// instead of heap String copies). This keeps lock hold times short in
/// `snapshot()` and `upsert()`.
#[derive(Clone, PartialEq, Eq)]
pub struct CachedIdentity {
    pub resource_kind: ResourceKind,
    pub namespace: Arc<str>,
    pub pod_name: Arc<str>,
    pub service_name: Arc<str>,
    pub node_name: Arc<str>,
    pub labels: Arc<[Arc<str>]>,
    pub workloads: Arc<[CachedWorkload]>,
}

/// Canonical operator state: maps IPs to identities and broadcasts changes.
pub struct OperatorState {
    cache: RwLock<HashMap<IpAddr, CachedIdentity>>,
    updates_tx: broadcast::Sender<IpCacheUpdate>,
    broadcast_capacity: usize,
    // Metrics.
    pub upserts_total: AtomicU64,
    pub upserts_skipped: AtomicU64,
    pub deletes_total: AtomicU64,
    pub connected_agents: AtomicU64,
}

impl OperatorState {
    pub fn new(broadcast_capacity: usize) -> Self {
        let (updates_tx, _) = broadcast::channel(broadcast_capacity);
        Self {
            cache: RwLock::new(HashMap::new()),
            updates_tx,
            broadcast_capacity,
            upserts_total: AtomicU64::new(0),
            upserts_skipped: AtomicU64::new(0),
            deletes_total: AtomicU64::new(0),
            connected_agents: AtomicU64::new(0),
        }
    }

    pub fn upsert(&self, ip: IpAddr, identity: CachedIdentity) {
        {
            let mut cache = self.cache.write().expect("lock poisoned");
            if cache.get(&ip) == Some(&identity) {
                self.upserts_skipped.fetch_add(1, Ordering::Relaxed);
                return; // No change, skip broadcast.
            }
            cache.insert(ip, identity.clone());
        }
        // Lock released. Build proto and broadcast outside the critical section.
        self.upserts_total.fetch_add(1, Ordering::Relaxed);
        let update = Self::to_proto(UpdateType::Upsert, &ip, &identity);
        // Ignore error if no subscribers.
        let _ = self.updates_tx.send(update);
    }

    /// Delete an IP entry, but only if owned by the given resource kind.
    ///
    /// This prevents cross-resource overwrites: e.g. a pod delete won't
    /// remove a node entry that shares the same IP.
    pub fn delete(&self, ip: &IpAddr, kind: ResourceKind) {
        let removed = {
            let mut cache = self.cache.write().expect("lock poisoned");
            match cache.get(ip) {
                Some(existing) if existing.resource_kind == kind => cache.remove(ip).is_some(),
                _ => false,
            }
        };
        if removed {
            self.deletes_total.fetch_add(1, Ordering::Relaxed);
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
        // Clone under the lock (cheap: Arc refcount bumps only).
        let entries: Vec<_> = self
            .cache
            .read()
            .expect("lock poisoned")
            .iter()
            .map(|(ip, id)| (*ip, id.clone()))
            .collect();
        // Lock released. Build protos without holding it.
        entries
            .iter()
            .map(|(ip, id)| Self::to_proto(UpdateType::Upsert, ip, id))
            .collect()
    }

    /// Subscribe to incremental updates. Must be called BEFORE snapshot()
    /// to avoid missing updates between snapshot and subscribe.
    pub fn subscribe(&self) -> broadcast::Receiver<IpCacheUpdate> {
        self.updates_tx.subscribe()
    }

    /// Broadcast a SHUTDOWN message to all connected agents so they can
    /// preserve their cache instead of clearing it on disconnect.
    pub fn broadcast_shutdown(&self) {
        let update = IpCacheUpdate {
            update_type: UpdateType::Shutdown.into(),
            ..Default::default()
        };
        let _ = self.updates_tx.send(update);
    }

    /// Return the current broadcast queue depth.
    pub fn broadcast_queue_depth(&self) -> usize {
        self.updates_tx.len()
    }

    /// Return the broadcast channel capacity.
    pub fn broadcast_capacity(&self) -> usize {
        self.broadcast_capacity
    }

    /// Return a snapshot of all entries for debugging.
    pub fn dump(&self) -> Vec<(IpAddr, CachedIdentity)> {
        self.cache
            .read()
            .expect("lock poisoned")
            .iter()
            .map(|(ip, id)| (*ip, id.clone()))
            .collect()
    }

    /// Convert a cached identity to a proto update message.
    fn to_proto(update_type: UpdateType, ip: &IpAddr, id: &CachedIdentity) -> IpCacheUpdate {
        IpCacheUpdate {
            update_type: update_type.into(),
            ip: ip.to_string(),
            namespace: id.namespace.to_string(),
            pod_name: id.pod_name.to_string(),
            service_name: id.service_name.to_string(),
            node_name: id.node_name.to_string(),
            labels: id.labels.iter().map(|l| l.to_string()).collect(),
            workloads: id
                .workloads
                .iter()
                .map(|w| ProtoWorkload {
                    name: w.name.to_string(),
                    kind: w.kind.to_string(),
                })
                .collect(),
        }
    }
}
