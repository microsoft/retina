use std::collections::HashMap;
use std::hash::{Hash, Hasher};
use std::net::IpAddr;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::RwLock;
use tokio::sync::{broadcast, Notify};

/// Change event emitted by the IpCache when entries are modified.
#[derive(Clone, Debug)]
pub enum IpCacheEvent {
    Upsert(IpAddr, Identity),
    Delete(IpAddr),
    Clear,
}

// Reserved numeric identities (matching Cilium conventions).
pub const IDENTITY_UNKNOWN: u32 = 0;
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
#[derive(Debug, Clone)]
pub struct Identity {
    pub namespace: String,
    pub pod_name: String,
    pub service_name: String,
    pub node_name: String,
    pub labels: Vec<String>,
    pub workloads: Vec<Workload>,
}

impl Identity {
    /// Compute a Cilium-compatible numeric identity from this identity's labels.
    ///
    /// - Nodes → `IDENTITY_REMOTE_NODE` (6)
    /// - Pods → hash identity-relevant labels into \[256, 65535\]
    /// - Services → hash namespace + service name into \[256, 65535\]
    /// - Empty/unknown → `IDENTITY_WORLD` (2)
    pub fn numeric_identity(&self) -> u32 {
        if !self.node_name.is_empty() {
            return IDENTITY_REMOTE_NODE;
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
/// determinism, then hashed with SipHash. The result is mapped into the range.
fn hash_labels_to_identity<S: AsRef<str>>(namespace: &str, labels: &[S]) -> u32 {
    let mut relevant: Vec<&str> = labels
        .iter()
        .map(|l| l.as_ref())
        .filter(|l| {
            !IDENTITY_IRRELEVANT_LABELS
                .iter()
                .any(|prefix| l.starts_with(prefix))
        })
        .collect();
    relevant.sort();

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
    pub name: String,
    pub kind: String,
}

/// Thread-safe IP-to-identity cache populated by the operator stream.
pub struct IpCache {
    inner: RwLock<HashMap<IpAddr, Identity>>,
    synced: AtomicBool,
    synced_notify: Notify,
    local_node_name: RwLock<String>,
    event_tx: broadcast::Sender<IpCacheEvent>,
}

impl IpCache {
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

    /// Set the local node name. Used to distinguish Host (local) vs RemoteNode.
    pub fn set_local_node_name(&self, name: String) {
        *self.local_node_name.write().unwrap() = name;
    }

    /// Resolve the numeric identity for a cached identity, taking into account
    /// reserved identities that require context (local node, API server).
    ///
    /// - `default/kubernetes` service → `IDENTITY_KUBE_APISERVER` (7)
    /// - Local node → `IDENTITY_HOST` (1)
    /// - Remote node → `IDENTITY_REMOTE_NODE` (6)
    /// - Pods → hash labels into \[256, 65535\]
    /// - Other services → hash into \[256, 65535\]
    /// - Unknown → `IDENTITY_WORLD` (2)
    pub fn resolve_identity(&self, id: &Identity) -> u32 {
        // Kubernetes API server service.
        if id.namespace == "default" && id.service_name == "kubernetes" {
            return IDENTITY_KUBE_APISERVER;
        }

        // Local node vs remote node.
        if !id.node_name.is_empty() {
            let local = self.local_node_name.read().unwrap();
            if !local.is_empty() && *local == id.node_name {
                return IDENTITY_HOST;
            }
            return IDENTITY_REMOTE_NODE;
        }

        id.numeric_identity()
    }

    pub fn upsert(&self, ip: IpAddr, identity: Identity) {
        self.inner.write().unwrap().insert(ip, identity.clone());
        let _ = self.event_tx.send(IpCacheEvent::Upsert(ip, identity));
    }

    pub fn delete(&self, ip: &IpAddr) {
        if self.inner.write().unwrap().remove(ip).is_some() {
            let _ = self.event_tx.send(IpCacheEvent::Delete(*ip));
        }
    }

    pub fn get(&self, ip: &IpAddr) -> Option<Identity> {
        self.inner.read().unwrap().get(ip).cloned()
    }

    pub fn mark_synced(&self) {
        self.synced.store(true, Ordering::Release);
        self.synced_notify.notify_waiters();
    }

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
            _ = notified => self.is_synced(),
            _ = tokio::time::sleep(timeout) => self.is_synced(),
        }
    }

    pub fn len(&self) -> usize {
        self.inner.read().unwrap().len()
    }

    /// Return a snapshot of all entries for debugging.
    pub fn dump(&self) -> Vec<(IpAddr, Identity)> {
        self.inner
            .read()
            .unwrap()
            .iter()
            .map(|(ip, id)| (*ip, id.clone()))
            .collect()
    }

    /// Return all node entries as (node_name, ip) pairs.
    pub fn get_node_peers(&self) -> Vec<(String, IpAddr)> {
        let inner = self.inner.read().unwrap();
        let mut seen = std::collections::HashSet::new();
        inner
            .iter()
            .filter(|(_, id)| !id.node_name.is_empty())
            .filter(|(_, id)| seen.insert(id.node_name.clone()))
            .map(|(ip, id)| (id.node_name.clone(), *ip))
            .collect()
    }

    /// Clear all entries and mark as unsynced. Called on reconnect.
    pub fn clear(&self) {
        self.synced.store(false, Ordering::Release);
        self.inner.write().unwrap().clear();
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
                namespace: "default".into(),
                pod_name: "nginx-abc".into(),
                service_name: String::new(),
                node_name: String::new(),
                labels: vec!["app=nginx".into()],
                workloads: vec![Workload {
                    name: "nginx".into(),
                    kind: "Deployment".into(),
                }],
            },
        );
        let id = cache.get(&ip).unwrap();
        assert_eq!(id.namespace, "default");
        assert_eq!(id.pod_name, "nginx-abc");
        assert_eq!(id.labels, vec!["app=nginx"]);
    }

    #[test]
    fn delete_removes_entry() {
        let cache = IpCache::new();
        let ip = IpAddr::V4(Ipv4Addr::new(10, 0, 0, 2));
        cache.upsert(
            ip,
            Identity {
                namespace: "kube-system".into(),
                pod_name: "coredns".into(),
                service_name: String::new(),
                node_name: String::new(),
                labels: vec![],
                workloads: vec![],
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
            namespace: "default".into(),
            pod_name: "nginx-abc".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec!["app=nginx".into(), "tier=frontend".into()],
            workloads: vec![],
        };
        let num = id.numeric_identity();
        assert!(num >= MIN_CLUSTER_IDENTITY && num <= MAX_CLUSTER_IDENTITY);
    }

    #[test]
    fn same_labels_same_identity() {
        let id1 = Identity {
            namespace: "default".into(),
            pod_name: "nginx-abc".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec!["app=nginx".into(), "tier=frontend".into()],
            workloads: vec![],
        };
        let id2 = Identity {
            namespace: "default".into(),
            pod_name: "nginx-xyz".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec!["tier=frontend".into(), "app=nginx".into()], // different order
            workloads: vec![],
        };
        assert_eq!(id1.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn different_labels_likely_different_identity() {
        let id1 = Identity {
            namespace: "default".into(),
            pod_name: "nginx".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec!["app=nginx".into()],
            workloads: vec![],
        };
        let id2 = Identity {
            namespace: "default".into(),
            pod_name: "redis".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec!["app=redis".into()],
            workloads: vec![],
        };
        // Not guaranteed, but extremely unlikely to collide with SipHash.
        assert_ne!(id1.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn different_namespace_different_identity() {
        let id1 = Identity {
            namespace: "default".into(),
            pod_name: "nginx".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec!["app=nginx".into()],
            workloads: vec![],
        };
        let id2 = Identity {
            namespace: "production".into(),
            pod_name: "nginx".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec!["app=nginx".into()],
            workloads: vec![],
        };
        assert_ne!(id1.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn irrelevant_labels_ignored() {
        let id1 = Identity {
            namespace: "default".into(),
            pod_name: "nginx-abc".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec!["app=nginx".into()],
            workloads: vec![],
        };
        let id2 = Identity {
            namespace: "default".into(),
            pod_name: "nginx-xyz".into(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec![
                "app=nginx".into(),
                "pod-template-hash=abc123".into(),
                "controller-revision-hash=xyz789".into(),
            ],
            workloads: vec![],
        };
        assert_eq!(id1.numeric_identity(), id2.numeric_identity());
    }

    #[test]
    fn node_identity_is_remote_node() {
        let id = Identity {
            namespace: String::new(),
            pod_name: String::new(),
            service_name: String::new(),
            node_name: "node-1".into(),
            labels: vec![],
            workloads: vec![],
        };
        assert_eq!(id.numeric_identity(), IDENTITY_REMOTE_NODE);
    }

    #[test]
    fn service_identity_in_cluster_range() {
        let id = Identity {
            namespace: "backend".into(),
            pod_name: String::new(),
            service_name: "redis".into(),
            node_name: String::new(),
            labels: vec![],
            workloads: vec![],
        };
        let num = id.numeric_identity();
        assert!(num >= MIN_CLUSTER_IDENTITY && num <= MAX_CLUSTER_IDENTITY);
    }

    #[test]
    fn empty_identity_is_world() {
        let id = Identity {
            namespace: String::new(),
            pod_name: String::new(),
            service_name: String::new(),
            node_name: String::new(),
            labels: vec![],
            workloads: vec![],
        };
        assert_eq!(id.numeric_identity(), IDENTITY_WORLD);
    }

    #[test]
    fn resolve_kubernetes_service_is_apiserver() {
        let cache = IpCache::new();
        let id = Identity {
            namespace: "default".into(),
            pod_name: String::new(),
            service_name: "kubernetes".into(),
            node_name: String::new(),
            labels: vec![],
            workloads: vec![],
        };
        assert_eq!(cache.resolve_identity(&id), IDENTITY_KUBE_APISERVER);
    }

    #[test]
    fn resolve_local_node_is_host() {
        let cache = IpCache::new();
        cache.set_local_node_name("my-node".into());
        let id = Identity {
            namespace: String::new(),
            pod_name: String::new(),
            service_name: String::new(),
            node_name: "my-node".into(),
            labels: vec![],
            workloads: vec![],
        };
        assert_eq!(cache.resolve_identity(&id), IDENTITY_HOST);
    }

    #[test]
    fn resolve_remote_node_stays_remote() {
        let cache = IpCache::new();
        cache.set_local_node_name("my-node".into());
        let id = Identity {
            namespace: String::new(),
            pod_name: String::new(),
            service_name: String::new(),
            node_name: "other-node".into(),
            labels: vec![],
            workloads: vec![],
        };
        assert_eq!(cache.resolve_identity(&id), IDENTITY_REMOTE_NODE);
    }

    #[test]
    fn resolve_node_without_local_name_is_remote() {
        let cache = IpCache::new();
        // local_node_name not set — all nodes are remote.
        let id = Identity {
            namespace: String::new(),
            pod_name: String::new(),
            service_name: String::new(),
            node_name: "node-1".into(),
            labels: vec![],
            workloads: vec![],
        };
        assert_eq!(cache.resolve_identity(&id), IDENTITY_REMOTE_NODE);
    }
}
