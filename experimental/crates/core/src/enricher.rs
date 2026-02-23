use retina_proto::flow;

use crate::ipcache::{
    IDENTITY_HOST, IDENTITY_KUBE_APISERVER, IDENTITY_REMOTE_NODE, IDENTITY_WORLD, IpCache,
};

/// Enrich a flow's source and destination endpoints from the IP cache.
///
/// If the cache is not yet synced (initial dump incomplete), this is a no-op
/// so that flows are never populated with stale data.
///
/// IPs not found in the cache are assigned the reserved World identity (2).
pub fn enrich_flow(flow: &mut flow::Flow, cache: &IpCache) {
    if !cache.is_synced() {
        return;
    }

    let Some(ip_header) = flow.ip.as_ref() else {
        return;
    };

    let src_ip: Option<std::net::IpAddr> = ip_header.source.parse().ok();
    let dst_ip: Option<std::net::IpAddr> = ip_header.destination.parse().ok();

    // Batch lookup: single lock for both IPs + single local_node_name read.
    let (src_id, dst_id) = match (src_ip.as_ref(), dst_ip.as_ref()) {
        (Some(s), Some(d)) => cache.get_pair(s, d),
        (Some(s), None) => (cache.get(s), None),
        (None, Some(d)) => (None, cache.get(d)),
        (None, None) => return,
    };
    let local_name = cache.local_node_name();

    if src_ip.is_some() {
        if let Some(id) = &src_id {
            flow.source_names = identity_names(id);
            flow.source = Some(identity_to_endpoint_with_local(id, cache, &local_name));
        } else {
            flow.source = Some(world_endpoint());
        }
    }

    if dst_ip.is_some() {
        if let Some(id) = &dst_id {
            flow.destination_names = identity_names(id);
            flow.destination = Some(identity_to_endpoint_with_local(id, cache, &local_name));
        } else {
            flow.destination = Some(world_endpoint());
        }
    }
}

/// Build the `source_names` / `destination_names` repeated string field.
/// Hubble compact format uses this to display identity (e.g. "default/nginx").
#[doc(hidden)]
pub fn identity_names(id: &crate::ipcache::Identity) -> Vec<String> {
    if !id.pod_name.is_empty() {
        let mut s = String::with_capacity(id.namespace.len() + 1 + id.pod_name.len());
        s.push_str(&id.namespace);
        s.push('/');
        s.push_str(&id.pod_name);
        vec![s]
    } else if !id.service_name.is_empty() {
        let mut s = String::with_capacity(id.namespace.len() + 1 + id.service_name.len());
        s.push_str(&id.namespace);
        s.push('/');
        s.push_str(&id.service_name);
        vec![s]
    } else if !id.node_name.is_empty() {
        vec![id.node_name.to_string()]
    } else {
        vec![]
    }
}

#[doc(hidden)]
pub fn identity_to_endpoint_with_local(
    id: &crate::ipcache::Identity,
    cache: &IpCache,
    local_node_name: &str,
) -> flow::Endpoint {
    let numeric_id = cache.resolve_identity_with_local(id, local_node_name);

    let mut labels: Vec<String> = id.labels.iter().map(|l| l.to_string()).collect();

    // For service IPs, encode service name as a label (matching Go enricher).
    if !id.service_name.is_empty() {
        labels.push(format!("k8s:io.kubernetes.svc.name={}", id.service_name));
    }

    // Append the reserved label matching the resolved identity.
    if let Some(reserved_label) = reserved_label_for(numeric_id) {
        labels.push(reserved_label.to_string());
    }

    flow::Endpoint {
        id: numeric_id,
        identity: numeric_id,
        namespace: id.namespace.to_string(),
        pod_name: if !id.pod_name.is_empty() {
            id.pod_name.to_string()
        } else if !id.node_name.is_empty() {
            id.node_name.to_string()
        } else {
            String::new()
        },
        labels,
        workloads: id
            .workloads
            .iter()
            .map(|w| flow::Workload {
                name: w.name.to_string(),
                kind: w.kind.to_string(),
            })
            .collect(),
        ..Default::default()
    }
}

/// Return the Cilium-style reserved label for a reserved numeric identity.
fn reserved_label_for(id: u32) -> Option<&'static str> {
    match id {
        IDENTITY_HOST => Some("reserved:host"),
        IDENTITY_WORLD => Some("reserved:world"),
        IDENTITY_REMOTE_NODE => Some("reserved:remote-node"),
        IDENTITY_KUBE_APISERVER => Some("reserved:kube-apiserver"),
        _ => None,
    }
}

/// Build a World endpoint for IPs not found in the cache.
fn world_endpoint() -> flow::Endpoint {
    flow::Endpoint {
        id: IDENTITY_WORLD,
        identity: IDENTITY_WORLD,
        labels: vec!["reserved:world".to_string()],
        ..Default::default()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::ipcache::{
        IDENTITY_HOST, IDENTITY_KUBE_APISERVER, IDENTITY_REMOTE_NODE, IDENTITY_WORLD, Identity,
        Workload,
    };
    use std::net::{IpAddr, Ipv4Addr};
    use std::sync::Arc;

    fn make_flow(src: &str, dst: &str) -> flow::Flow {
        flow::Flow {
            ip: Some(flow::Ip {
                source: src.into(),
                destination: dst.into(),
                ..Default::default()
            }),
            ..Default::default()
        }
    }

    #[test]
    fn enrich_both_endpoints() {
        let cache = IpCache::new();
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(10, 0, 0, 1)),
            Identity {
                namespace: Arc::from("default"),
                pod_name: Arc::from("client-abc"),
                service_name: Arc::from(""),
                node_name: Arc::from(""),
                labels: vec![Arc::from("app=client")].into(),
                workloads: vec![Workload {
                    name: Arc::from("client"),
                    kind: Arc::from("Deployment"),
                }]
                .into(),
            },
        );
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(10, 0, 0, 2)),
            Identity {
                namespace: Arc::from("backend"),
                pod_name: Arc::from("server-xyz"),
                service_name: Arc::from(""),
                node_name: Arc::from(""),
                labels: vec![Arc::from("app=server")].into(),
                workloads: vec![].into(),
            },
        );
        cache.mark_synced();

        let mut flow = make_flow("10.0.0.1", "10.0.0.2");
        enrich_flow(&mut flow, &cache);

        let src = flow.source.unwrap();
        assert_eq!(src.namespace, "default");
        assert_eq!(src.pod_name, "client-abc");
        assert_eq!(src.labels, vec!["app=client"]);
        assert_eq!(src.workloads.len(), 1);
        assert_eq!(src.workloads[0].name, "client");

        let dst = flow.destination.unwrap();
        assert_eq!(dst.namespace, "backend");
        assert_eq!(dst.pod_name, "server-xyz");

        assert_eq!(flow.source_names, vec!["default/client-abc"]);
        assert_eq!(flow.destination_names, vec!["backend/server-xyz"]);
    }

    #[test]
    fn skip_enrichment_when_not_synced() {
        let cache = IpCache::new();
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(10, 0, 0, 1)),
            Identity {
                namespace: Arc::from("default"),
                pod_name: Arc::from("pod"),
                service_name: Arc::from(""),
                node_name: Arc::from(""),
                labels: vec![].into(),
                workloads: vec![].into(),
            },
        );
        // Not calling mark_synced()

        let mut flow = make_flow("10.0.0.1", "10.0.0.2");
        enrich_flow(&mut flow, &cache);

        assert!(flow.source.is_none());
        assert!(flow.destination.is_none());
    }

    #[test]
    fn kubernetes_service_gets_apiserver_identity() {
        let cache = IpCache::new();
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(10, 96, 0, 1)),
            Identity {
                namespace: Arc::from("default"),
                pod_name: Arc::from(""),
                service_name: Arc::from("kubernetes"),
                node_name: Arc::from(""),
                labels: vec![].into(),
                workloads: vec![].into(),
            },
        );
        cache.mark_synced();

        let mut flow = make_flow("10.96.0.1", "10.0.0.2");
        enrich_flow(&mut flow, &cache);

        let src = flow.source.unwrap();
        assert_eq!(src.id, IDENTITY_KUBE_APISERVER);
        assert_eq!(src.identity, IDENTITY_KUBE_APISERVER);
        assert!(src.labels.contains(&"reserved:kube-apiserver".to_string()));
        assert!(
            src.labels
                .contains(&"k8s:io.kubernetes.svc.name=kubernetes".to_string())
        );
    }

    #[test]
    fn non_apiserver_service_gets_hashed_identity() {
        let cache = IpCache::new();
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(10, 96, 0, 10)),
            Identity {
                namespace: Arc::from("backend"),
                pod_name: Arc::from(""),
                service_name: Arc::from("redis"),
                node_name: Arc::from(""),
                labels: vec![].into(),
                workloads: vec![].into(),
            },
        );
        cache.mark_synced();

        let mut flow = make_flow("10.96.0.10", "10.0.0.2");
        enrich_flow(&mut flow, &cache);

        let src = flow.source.unwrap();
        assert!(src.id >= 256 && src.id <= 65535);
        assert_eq!(src.id, src.identity);
        // No reserved label for hashed identities.
        assert!(!src.labels.iter().any(|l| l.starts_with("reserved:")));
    }

    #[test]
    fn node_ip_uses_node_name_as_pod_name() {
        let cache = IpCache::new();
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(192, 168, 1, 10)),
            Identity {
                namespace: Arc::from(""),
                pod_name: Arc::from(""),
                service_name: Arc::from(""),
                node_name: Arc::from("node-1"),
                labels: vec![].into(),
                workloads: vec![].into(),
            },
        );
        cache.mark_synced();

        let mut flow = make_flow("192.168.1.10", "10.0.0.1");
        enrich_flow(&mut flow, &cache);

        let src = flow.source.unwrap();
        assert_eq!(src.pod_name, "node-1");
    }

    #[test]
    fn unknown_ip_gets_world_identity() {
        let cache = IpCache::new();
        cache.mark_synced();

        let mut flow = make_flow("10.0.0.99", "10.0.0.100");
        enrich_flow(&mut flow, &cache);

        let src = flow.source.unwrap();
        assert_eq!(src.id, IDENTITY_WORLD);
        assert_eq!(src.identity, IDENTITY_WORLD);
        assert!(src.labels.contains(&"reserved:world".to_string()));

        let dst = flow.destination.unwrap();
        assert_eq!(dst.id, IDENTITY_WORLD);
        assert_eq!(dst.identity, IDENTITY_WORLD);
    }

    #[test]
    fn pod_endpoint_gets_numeric_identity() {
        let cache = IpCache::new();
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(10, 0, 0, 1)),
            Identity {
                namespace: Arc::from("default"),
                pod_name: Arc::from("nginx-abc"),
                service_name: Arc::from(""),
                node_name: Arc::from(""),
                labels: vec![Arc::from("app=nginx"), Arc::from("tier=frontend")].into(),
                workloads: vec![].into(),
            },
        );
        cache.mark_synced();

        let mut flow = make_flow("10.0.0.1", "10.0.0.2");
        enrich_flow(&mut flow, &cache);

        let src = flow.source.unwrap();
        // Must be in cluster-local range [256, 65535].
        assert!(src.id >= 256 && src.id <= 65535);
        assert_eq!(src.id, src.identity);
    }

    #[test]
    fn remote_node_gets_remote_node_identity() {
        let cache = IpCache::new();
        cache.set_local_node_name("my-node".into());
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(192, 168, 1, 10)),
            Identity {
                namespace: Arc::from(""),
                pod_name: Arc::from(""),
                service_name: Arc::from(""),
                node_name: Arc::from("node-1"),
                labels: vec![].into(),
                workloads: vec![].into(),
            },
        );
        cache.mark_synced();

        let mut flow = make_flow("192.168.1.10", "10.0.0.1");
        enrich_flow(&mut flow, &cache);

        let src = flow.source.unwrap();
        assert_eq!(src.id, IDENTITY_REMOTE_NODE);
        assert_eq!(src.identity, IDENTITY_REMOTE_NODE);
        assert!(src.labels.contains(&"reserved:remote-node".to_string()));
    }

    #[test]
    fn local_node_gets_host_identity() {
        let cache = IpCache::new();
        cache.set_local_node_name("my-node".into());
        cache.upsert(
            IpAddr::V4(Ipv4Addr::new(192, 168, 1, 5)),
            Identity {
                namespace: Arc::from(""),
                pod_name: Arc::from(""),
                service_name: Arc::from(""),
                node_name: Arc::from("my-node"),
                labels: vec![].into(),
                workloads: vec![].into(),
            },
        );
        cache.mark_synced();

        let mut flow = make_flow("192.168.1.5", "10.0.0.1");
        enrich_flow(&mut flow, &cache);

        let src = flow.source.unwrap();
        assert_eq!(src.id, IDENTITY_HOST);
        assert_eq!(src.identity, IDENTITY_HOST);
        assert!(src.labels.contains(&"reserved:host".to_string()));
        assert_eq!(src.pod_name, "my-node");
    }
}
