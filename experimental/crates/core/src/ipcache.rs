//! Thread-safe IP-to-identity cache with broadcast notifications.
//! Maps IP addresses to Kubernetes identity metadata (pod, service, node).

use std::collections::HashMap;
use std::hash::{Hash, Hasher};
use std::net::IpAddr;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, RwLock};
use tokio::sync::{Notify, broadcast};

/// Change event emitted by the `IpCache` when entries are modified.
#[derive(Clone, Debug)]
pub enum IpCacheEvent {
    Upsert(IpAddr, Identity),
    Delete(IpAddr),
    Clear,
}

// Reserved numeric identities (matching Cilium conventions).
pub const IDENTITY_HOST: u32 = 1;
pub const IDENTITY_WORLD: u32 = 2;
pub const IDENTITY_REMOTE_NODE: u32 = 6;
pub const IDENTITY_KUBE_APISERVER: u32 = 7;

// Cluster-local identity range for label-derived identities.
const MIN_CLUSTER_IDENTITY: u32 = 256;
const MAX_CLUSTER_IDENTITY: u32 = 65535;
const CLUSTER_IDENTITY_RANGE: u32 = MAX_CLUSTER_IDENTITY - MIN_CLUSTER_IDENTITY + 1;

/// Label prefixes that are not identity-relevant (high-cardinality or ephemeral).
const IDENTITY_IRRELEVANT_LABELS: &[&str] = &[
    "pod-template-hash=",
    "controller-revision-hash=",
    "pod-template-generation=",
    "statefulset.kubernetes.io/pod-name=",
    "batch.kubernetes.io/job-completion-index=",
];

/// Kubernetes identity associated with an IP address.
///
/// Fields use `Arc<str>` so that cloning an Identity (on every packet lookup)
/// is nearly free (atomic ref bumps instead of heap String copies).
#[derive(Debug, Clone)]
pub struct Identity {
    pub namespace: Arc<str>,
    pub pod_name: Arc<str>,
    pub service_name: Arc<str>,
    pub node_name: Arc<str>,
    pub labels: Arc<[Arc<str>]>,
    pub workloads: Arc<[Workload]>,
}

impl Identity {
    /// Compute a Cilium-compatible numeric identity from this identity's labels.
    ///
    /// - Nodes → `IDENTITY_REMOTE_NODE` (6)
    /// - Pods → hash identity-relevant labels into \[256, 65535\]
    /// - Services → hash namespace + service name into \[256, 65535\]
    /// - Empty/unknown → `IDENTITY_WORLD` (2)
    #[must_use]
    pub fn numeric_identity(&self) -> u32 {
        if !self.node_name.is_empty() {
            return hash_labels_to_identity(
                "",
                &[format!("k8s:io.kubernetes.node.name={}", self.node_name)],
            );
        }

        if !self.pod_name.is_empty() {
            return hash_labels_to_identity(&self.namespace, &self.labels);
        }

        if !self.service_name.is_empty() {
            // Services don't carry pod labels; hash namespace + name.
            let synthetic = format!("k8s:io.kubernetes.svc.name={}", self.service_name);
            return hash_labels_to_identity(&self.namespace, &[synthetic]);
        }

        IDENTITY_WORLD
    }
}

/// Hash a namespace + set of labels into the cluster-local identity range [256, 65535].
///
/// Labels are filtered to remove high-cardinality/ephemeral keys, sorted for
/// determinism, then hashed with `SipHash`. The result is mapped into the range.
fn hash_labels_to_identity<S: AsRef<str>>(namespace: &str, labels: &[S]) -> u32 {
    let mut relevant: Vec<&str> = labels
        .iter()
        .map(std::convert::AsRef::as_ref)
        .filter(|l| {
            !IDENTITY_IRRELEVANT_LABELS
                .iter()
                .any(|prefix| l.starts_with(prefix))
        })
        .collect();
    relevant.sort_unstable();

    let mut hasher = std::collections::hash_map::DefaultHasher::new();
    namespace.hash(&mut hasher);
    for label in &relevant {
        label.hash(&mut hasher);
    }
    let hash = hasher.finish();
    (hash as u32 % CLUSTER_IDENTITY_RANGE) + MIN_CLUSTER_IDENTITY
}

#[derive(Debug, Clone)]
pub struct Workload {
    pub name: Arc<str>,
    pub kind: Arc<str>,
}

/// Thread-safe IP-to-identity cache populated by the operator stream.
pub struct IpCache {
    inner: RwLock<HashMap<IpAddr, Identity>>,
    synced: AtomicBool,
    synced_notify: Notify,
    local_node_name: RwLock<String>,
    event_tx: broadcast::Sender<IpCacheEvent>,
}

impl Default for IpCache {
    fn default() -> Self {
        Self::new()
    }
}

impl IpCache {
    #[must_use]
    pub fn new() -> Self {
        let (event_tx, _) = broadcast::channel(4096);
        Self {
            inner: RwLock::new(HashMap::new()),
            synced: AtomicBool::new(false),
            synced_notify: Notify::new(),
            local_node_name: RwLock::new(String::new()),
            event_tx,
        }
    }

    /// Subscribe to change notifications from this cache.
    pub fn subscribe(&self) -> broadcast::Receiver<IpCacheEvent> {
        self.event_tx.subscribe()
    }

    /// Set the local node name. Used to distinguish Host (local) vs `RemoteNode`.
    pub fn set_local_node_name(&self, name: String) {
        *self.local_node_name.write().expect("lock poisoned") = name;
    }

    pub fn upsert(&self, ip: IpAddr, identity: Identity) {
        self.inner
            .write()
            .expect("lock poisoned")
            .insert(ip, identity.clone());
        let _ = self.event_tx.send(IpCacheEvent::Upsert(ip, identity));
    }

    pub fn delete(&self, ip: &IpAddr) {
        if self
            .inner
            .write()
            .expect("lock poisoned")
            .remove(ip)
            .is_some()
        {
            let _ = self.event_tx.send(IpCacheEvent::Delete(*ip));
        }
    }

    #[must_use]
    pub fn get(&self, ip: &IpAddr) -> Option<Identity> {
        self.inner.read().expect("lock poisoned").get(ip).cloned()
    }

    /// Look up two IPs in a single lock acquisition (hot-path optimization).
    #[must_use]
    pub fn get_pair(&self, ip1: &IpAddr, ip2: &IpAddr) -> (Option<Identity>, Option<Identity>) {
        let inner = self.inner.read().expect("lock poisoned");
        (inner.get(ip1).cloned(), inner.get(ip2).cloned())
    }

    /// Return the local node name.
    pub fn local_node_name(&self) -> String {
        self.local_node_name.read().expect("lock poisoned").clone()
    }

    /// Resolve numeric identity. The `local_node_name` parameter is unused
    /// here (node host/remote distinction is now in enricher labels) but kept
    /// in the signature for API compatibility with callers that also need it.
    #[must_use]
    pub fn resolve_identity_with_local(&self, id: &Identity, _local_node_name: &str) -> u32 {
        if &*id.namespace == "default" && &*id.service_name == "kubernetes" {
            return IDENTITY_KUBE_APISERVER;
        }
        id.numeric_identity()
    }

    pub fn mark_synced(&self) {
        self.synced.store(true, Ordering::Release);
        self.synced_notify.notify_waiters();
    }

    #[must_use]
    pub fn is_synced(&self) -> bool {
        self.synced.load(Ordering::Acquire)
    }

    /// Wait until the cache has completed its initial sync, or the timeout expires.
    pub async fn wait_synced(&self, timeout: std::time::Duration) -> bool {
        if self.is_synced() {
            return true;
        }
        // Register interest BEFORE re-checking, so a mark_synced() call
        // between the check and the await is not lost.
        let notified = self.synced_notify.notified();
        if self.is_synced() {
            return true;
        }
        tokio::select! {
            () = notified => self.is_synced(),
            () = tokio::time::sleep(timeout) => self.is_synced(),
        }
    }

    #[allow(clippy::len_without_is_empty)]
    pub fn len(&self) -> usize {
        self.inner.read().expect("lock poisoned").len()
    }

    /// Return a snapshot of all entries for debugging.
    pub fn dump(&self) -> Vec<(IpAddr, Identity)> {
        self.inner
            .read()
            .expect("lock poisoned")
            .iter()
            .map(|(ip, id)| (*ip, id.clone()))
            .collect()
    }

    /// Return all node entries as (`node_name`, ip) pairs.
    pub fn get_node_peers(&self) -> Vec<(String, IpAddr)> {
        let inner = self.inner.read().expect("lock poisoned");
        let mut seen = std::collections::HashSet::new();
        inner
            .iter()
            .filter(|(_, id)| !id.node_name.is_empty())
            .filter(|(_, id)| seen.insert(id.node_name.clone()))
            .map(|(ip, id)| (id.node_name.to_string(), *ip))
            .collect()
    }

    /// Clear all entries and mark as unsynced. Called on reconnect.
    pub fn clear(&self) {
        self.synced.store(false, Ordering::Release);
        self.inner.write().expect("lock poisoned").clear();
        let _ = self.event_tx.send(IpCacheEvent::Clear);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::Ipv4Addr;

    #[test]
    fn upsert_and_get() {
        let cache = IpCache::new();
        let ip = IpAddr::V4(Ipv4Addr::new(10, 0, 0, 1));
        cache.upsert(
            ip,
            Identity {
                namespace: Arc::from("default"),
                pod_name: Arc::from("nginx-abc"),
                service_name: Arc::from(""),
                node_name: Arc::from(""),
                labels: vec![Arc::from("app=nginx")].into(),
                workloads: vec![Workload {
                    name: Arc::from("nginx"),
                    kind: Arc::from("Deployment"),
                }]
                .into(),
            },
        );
        let id = cache.get(&ip).unwrap();
        assert_eq!(&*id.namespace, "default");
        assert_eq!(&*id.pod_name, "nginx-abc");
        assert_eq!(id.labels.len(), 1);
        assert_eq!(&*id.labels[0], "app=nginx");
    }

    #[test]
    fn delete_removes_entry() {
        let cache = IpCache::new();
        let ip = IpAddr::V4(Ipv4Addr::new(10, 0, 0, 2));
        cache.upsert(
            ip,
            Identity {
                namespace: Arc::from("kube-system"),
                pod_name: Arc::from("coredns"),
                service_name: Arc::from(""),
                node_name: Arc::from(""),
                labels: vec![].into(),
                workloads: vec![].into(),
            },
        );
        cache.delete(&ip);
        assert!(cache.get(&ip).is_none());
    }

    #[test]
    fn sync_state() {
        let cache = IpCache::new();
        assert!(!cache.is_synced());
        cache.mark_synced();
        assert!(cache.is_synced());
        cache.clear();
        assert!(!cache.is_synced());
    }

    #[test]
    fn pod_numeric_identity_in_cluster_range() {
        let id = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from("nginx-abc"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![Arc::from("app=nginx"), Arc::from("tier=frontend")].into(),
            workloads: vec![].into(),
        };
        let num = id.numeric_identity();
        assert!(num >= MIN_CLUSTER_IDENTITY && num <= MAX_CLUSTER_IDENTITY);
    }

    #[test]
    fn same_labels_same_identity() {
        let id1 = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from("nginx-abc"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![Arc::from("app=nginx"), Arc::from("tier=frontend")].into(),
            workloads: vec![].into(),
        };
        let id2 = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from("nginx-xyz"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![Arc::from("tier=frontend"), Arc::from("app=nginx")].into(), // different order
            workloads: vec![].into(),
        };
        assert_eq!(id1.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn different_labels_likely_different_identity() {
        let id1 = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from("nginx"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![Arc::from("app=nginx")].into(),
            workloads: vec![].into(),
        };
        let id2 = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from("redis"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![Arc::from("app=redis")].into(),
            workloads: vec![].into(),
        };
        // Not guaranteed, but extremely unlikely to collide with SipHash.
        assert_ne!(id1.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn different_namespace_different_identity() {
        let id1 = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from("nginx"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![Arc::from("app=nginx")].into(),
            workloads: vec![].into(),
        };
        let id2 = Identity {
            namespace: Arc::from("production"),
            pod_name: Arc::from("nginx"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![Arc::from("app=nginx")].into(),
            workloads: vec![].into(),
        };
        assert_ne!(id1.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn irrelevant_labels_ignored() {
        let id1 = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from("nginx-abc"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![Arc::from("app=nginx")].into(),
            workloads: vec![].into(),
        };
        let id2 = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from("nginx-xyz"),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![
                Arc::from("app=nginx"),
                Arc::from("pod-template-hash=abc123"),
                Arc::from("controller-revision-hash=xyz789"),
            ]
            .into(),
            workloads: vec![].into(),
        };
        assert_eq!(id1.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn node_identity_is_unique_per_node() {
        let id = Identity {
            namespace: Arc::from(""),
            pod_name: Arc::from(""),
            service_name: Arc::from(""),
            node_name: Arc::from("node-1"),
            labels: vec![].into(),
            workloads: vec![].into(),
        };
        let num = id.numeric_identity();
        assert!(num >= MIN_CLUSTER_IDENTITY && num <= MAX_CLUSTER_IDENTITY);

        let id2 = Identity {
            namespace: Arc::from(""),
            pod_name: Arc::from(""),
            service_name: Arc::from(""),
            node_name: Arc::from("node-2"),
            labels: vec![].into(),
            workloads: vec![].into(),
        };
        assert_ne!(id.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn service_identity_in_cluster_range() {
        let id = Identity {
            namespace: Arc::from("backend"),
            pod_name: Arc::from(""),
            service_name: Arc::from("redis"),
            node_name: Arc::from(""),
            labels: vec![].into(),
            workloads: vec![].into(),
        };
        let num = id.numeric_identity();
        assert!(num >= MIN_CLUSTER_IDENTITY && num <= MAX_CLUSTER_IDENTITY);
    }

    #[test]
    fn empty_identity_is_world() {
        let id = Identity {
            namespace: Arc::from(""),
            pod_name: Arc::from(""),
            service_name: Arc::from(""),
            node_name: Arc::from(""),
            labels: vec![].into(),
            workloads: vec![].into(),
        };
        assert_eq!(id.numeric_identity(), IDENTITY_WORLD);
    }

    #[test]
    fn resolve_kubernetes_service_is_apiserver() {
        let cache = IpCache::new();
        let id = Identity {
            namespace: Arc::from("default"),
            pod_name: Arc::from(""),
            service_name: Arc::from("kubernetes"),
            node_name: Arc::from(""),
            labels: vec![].into(),
            workloads: vec![].into(),
        };
        assert_eq!(
            cache.resolve_identity_with_local(&id, ""),
            IDENTITY_KUBE_APISERVER
        );
    }

    #[test]
    fn resolve_node_uses_hashed_identity() {
        let cache = IpCache::new();
        let id = Identity {
            namespace: Arc::from(""),
            pod_name: Arc::from(""),
            service_name: Arc::from(""),
            node_name: Arc::from("my-node"),
            labels: vec![].into(),
            workloads: vec![].into(),
        };
        // Local and remote nodes get the same hashed identity (distinction
        // is now in enricher labels, not the numeric ID).
        let expected = id.numeric_identity();
        assert!(expected >= MIN_CLUSTER_IDENTITY && expected <= MAX_CLUSTER_IDENTITY);
        assert_eq!(
            cache.resolve_identity_with_local(&id, "my-node"),
            expected
        );
        assert_eq!(
            cache.resolve_identity_with_local(&id, "other-node"),
            expected
        );
        assert_eq!(cache.resolve_identity_with_local(&id, ""), expected);
    }
}
