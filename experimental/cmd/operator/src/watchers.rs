use std::net::IpAddr;
use std::sync::Arc;

use futures::TryStreamExt;
use k8s_openapi::api::core::v1::{Node, Pod, Service};
use kube::runtime::watcher::{self, Event};
use kube::{Api, Client};
use retina_core::retry::retry_with_backoff;
use retina_proto::ipcache::Workload;
use tracing::{debug, warn};

use crate::state::{CachedIdentity, OperatorState};

/// Watch all pods cluster-wide and upsert/delete their IPs.
/// Automatically restarts with backoff on stream errors.
pub async fn watch_pods(client: Client, state: Arc<OperatorState>) {
    retry_with_backoff("pod watcher", || {
        try_watch_pods(client.clone(), state.clone())
    })
    .await
}

async fn try_watch_pods(client: Client, state: Arc<OperatorState>) -> anyhow::Result<()> {
    let pods: Api<Pod> = Api::all(client);
    let stream = watcher::watcher(pods, watcher::Config::default());

    futures::pin_mut!(stream);

    while let Some(event) = stream.try_next().await? {
        match event {
            Event::Apply(pod) | Event::InitApply(pod) => {
                handle_pod_apply(&pod, &state);
            }
            Event::Delete(pod) => {
                handle_pod_delete(&pod, &state);
            }
            Event::Init | Event::InitDone => {}
        }
    }

    Ok(())
}

fn handle_pod_apply(pod: &Pod, state: &OperatorState) {
    let meta = &pod.metadata;
    let name = meta.name.as_deref().unwrap_or_default();
    let namespace = meta.namespace.as_deref().unwrap_or_default();

    let status = match pod.status.as_ref() {
        Some(s) => s,
        None => return,
    };

    // Skip pods with host networking.
    if pod
        .spec
        .as_ref()
        .and_then(|s| s.host_network)
        .unwrap_or(false)
    {
        return;
    }

    let pod_ips = match status.pod_ips.as_ref() {
        Some(ips) => ips,
        None => return,
    };

    // Build labels as "key=value".
    let labels: Vec<String> = meta
        .labels
        .as_ref()
        .map(|m| m.iter().map(|(k, v)| format!("{k}={v}")).collect())
        .unwrap_or_default();

    // Extract workloads from owner references.
    let workloads: Vec<Workload> = meta
        .owner_references
        .as_ref()
        .map(|refs| {
            refs.iter()
                .filter(|r| {
                    matches!(
                        r.kind.as_str(),
                        "ReplicaSet" | "Deployment" | "StatefulSet" | "DaemonSet" | "Job"
                    )
                })
                .map(|r| Workload {
                    name: r.name.clone(),
                    kind: r.kind.clone(),
                })
                .collect()
        })
        .unwrap_or_default();

    for pod_ip in pod_ips {
        let ip_str = &pod_ip.ip;
        if ip_str.is_empty() {
            continue;
        }
        let ip: IpAddr = match ip_str.parse() {
            Ok(ip) => ip,
            Err(e) => {
                warn!(pod = name, ip = ip_str, "invalid pod IP: {}", e);
                continue;
            }
        };

        debug!(pod = name, ns = namespace, %ip, "upsert pod");
        state.upsert(
            ip,
            CachedIdentity {
                namespace: namespace.to_string(),
                pod_name: name.to_string(),
                service_name: String::new(),
                node_name: String::new(),
                labels: labels.clone(),
                workloads: workloads.clone(),
            },
        );
    }
}

fn handle_pod_delete(pod: &Pod, state: &OperatorState) {
    let name = pod.metadata.name.as_deref().unwrap_or_default();

    // Skip pods with host networking — their IPs are node IPs managed by
    // the node watcher. Deleting here would remove the node entry.
    if pod
        .spec
        .as_ref()
        .and_then(|s| s.host_network)
        .unwrap_or(false)
    {
        return;
    }

    if let Some(status) = pod.status.as_ref()
        && let Some(pod_ips) = status.pod_ips.as_ref()
    {
        for pod_ip in pod_ips {
            if !pod_ip.ip.is_empty()
                && let Ok(ip) = pod_ip.ip.parse::<IpAddr>()
            {
                debug!(pod = name, %ip, "delete pod");
                state.delete(&ip);
            }
        }
    }
}

/// Watch all services cluster-wide and upsert/delete their ClusterIP and LB IPs.
/// Automatically restarts with backoff on stream errors.
pub async fn watch_services(client: Client, state: Arc<OperatorState>) {
    retry_with_backoff("service watcher", || {
        try_watch_services(client.clone(), state.clone())
    })
    .await
}

async fn try_watch_services(client: Client, state: Arc<OperatorState>) -> anyhow::Result<()> {
    let services: Api<Service> = Api::all(client);
    let stream = watcher::watcher(services, watcher::Config::default());

    futures::pin_mut!(stream);

    while let Some(event) = stream.try_next().await? {
        match event {
            Event::Apply(svc) | Event::InitApply(svc) => {
                handle_service_apply(&svc, &state);
            }
            Event::Delete(svc) => {
                handle_service_delete(&svc, &state);
            }
            Event::Init | Event::InitDone => {}
        }
    }

    Ok(())
}

fn service_ips(svc: &Service) -> Vec<IpAddr> {
    let mut ips = Vec::new();

    if let Some(spec) = svc.spec.as_ref() {
        // ClusterIP (skip headless "None").
        if let Some(cluster_ip) = spec.cluster_ip.as_deref()
            && cluster_ip != "None"
            && let Ok(ip) = cluster_ip.parse()
        {
            ips.push(ip);
        }
    }

    // LoadBalancer IPs.
    if let Some(status) = svc.status.as_ref()
        && let Some(lb) = status.load_balancer.as_ref()
        && let Some(ingresses) = lb.ingress.as_ref()
    {
        for ingress in ingresses {
            if let Some(ip_str) = ingress.ip.as_deref()
                && let Ok(ip) = ip_str.parse()
            {
                ips.push(ip);
            }
        }
    }

    ips
}

fn handle_service_apply(svc: &Service, state: &OperatorState) {
    let meta = &svc.metadata;
    let name = meta.name.as_deref().unwrap_or_default();
    let namespace = meta.namespace.as_deref().unwrap_or_default();

    for ip in service_ips(svc) {
        debug!(svc = name, ns = namespace, %ip, "upsert service");
        state.upsert(
            ip,
            CachedIdentity {
                namespace: namespace.to_string(),
                pod_name: String::new(),
                service_name: name.to_string(),
                node_name: String::new(),
                labels: Vec::new(),
                workloads: Vec::new(),
            },
        );
    }
}

fn handle_service_delete(svc: &Service, state: &OperatorState) {
    let name = svc.metadata.name.as_deref().unwrap_or_default();
    for ip in service_ips(svc) {
        debug!(svc = name, %ip, "delete service");
        state.delete(&ip);
    }
}

/// Watch all nodes and upsert/delete their InternalIP addresses.
/// Automatically restarts with backoff on stream errors.
pub async fn watch_nodes(client: Client, state: Arc<OperatorState>) {
    retry_with_backoff("node watcher", || {
        try_watch_nodes(client.clone(), state.clone())
    })
    .await
}

async fn try_watch_nodes(client: Client, state: Arc<OperatorState>) -> anyhow::Result<()> {
    let nodes: Api<Node> = Api::all(client);
    let stream = watcher::watcher(nodes, watcher::Config::default());

    futures::pin_mut!(stream);

    while let Some(event) = stream.try_next().await? {
        match event {
            Event::Apply(node) | Event::InitApply(node) => {
                handle_node_apply(&node, &state);
            }
            Event::Delete(node) => {
                handle_node_delete(&node, &state);
            }
            Event::Init | Event::InitDone => {}
        }
    }

    Ok(())
}

/// Collect all IPs that represent this node: InternalIP + pod CIDR gateway IPs.
fn node_ips(node: &Node) -> Vec<IpAddr> {
    let mut ips = Vec::new();

    // InternalIP from status.addresses.
    if let Some(status) = node.status.as_ref()
        && let Some(addresses) = status.addresses.as_ref()
    {
        for addr in addresses {
            if addr.type_ == "InternalIP"
                && let Ok(ip) = addr.address.parse()
            {
                ips.push(ip);
            }
        }
    }

    // Pod CIDR gateway IPs (first usable IP in each CIDR, e.g. 10.244.0.1).
    // The node uses this IP as the gateway on veth pairs to pods.
    if let Some(spec) = node.spec.as_ref()
        && let Some(cidrs) = spec.pod_cidrs.as_ref()
    {
        for cidr in cidrs {
            if let Some(gw) = pod_cidr_gateway(cidr) {
                ips.push(gw);
            }
        }
    }

    ips
}

/// Parse a CIDR string and return the first usable IP (gateway).
/// e.g. "10.244.0.0/24" → 10.244.0.1, "fd00:10:244::/64" → fd00:10:244::1
fn pod_cidr_gateway(cidr: &str) -> Option<IpAddr> {
    let (ip_str, _) = cidr.split_once('/')?;
    let ip: IpAddr = ip_str.parse().ok()?;
    match ip {
        IpAddr::V4(v4) => {
            let bits = u32::from(v4);
            Some(IpAddr::V4((bits + 1).into()))
        }
        IpAddr::V6(v6) => {
            let bits = u128::from(v6);
            Some(IpAddr::V6((bits + 1).into()))
        }
    }
}

fn handle_node_apply(node: &Node, state: &OperatorState) {
    let name = node.metadata.name.as_deref().unwrap_or_default();
    for ip in node_ips(node) {
        debug!(node = name, %ip, "upsert node");
        state.upsert(
            ip,
            CachedIdentity {
                namespace: String::new(),
                pod_name: String::new(),
                service_name: String::new(),
                node_name: name.to_string(),
                labels: Vec::new(),
                workloads: Vec::new(),
            },
        );
    }
}

fn handle_node_delete(node: &Node, state: &OperatorState) {
    let name = node.metadata.name.as_deref().unwrap_or_default();
    for ip in node_ips(node) {
        debug!(node = name, %ip, "delete node");
        state.delete(&ip);
    }
}
