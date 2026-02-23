use std::collections::BTreeSet;
use std::sync::Arc;
use std::sync::atomic::Ordering;

use prost_types::Timestamp;
use retina_core::filter::FlowFilterSet;
use retina_core::ipcache::{IpCache, IpCacheEvent};
use retina_core::metrics::AgentState;
use retina_core::store::AgentEventStore;
use retina_core::store::FlowStore;
use retina_proto::{
    flow::{AgentEvent, Flow},
    observer::{
        GetAgentEventsRequest, GetAgentEventsResponse, GetDebugEventsRequest,
        GetDebugEventsResponse, GetFlowsRequest, GetFlowsResponse, GetNamespacesRequest,
        GetNamespacesResponse, GetNodesRequest, GetNodesResponse, Namespace, Node,
        ServerStatusRequest, ServerStatusResponse, Tls,
        observer_server::{Observer, ObserverServer},
    },
    peer::{
        ChangeNotification, ChangeNotificationType, NotifyRequest,
        peer_server::{Peer, PeerServer},
    },
    relay::NodeState,
};
use tokio::sync::broadcast;
use tokio_stream::wrappers::ReceiverStream;
use tonic::codec::CompressionEncoding;
use tonic::{Request, Response, Status};
use tracing::info;

const GRPC_CHANNEL_CAPACITY: usize = 256;
const PEER_CHANNEL_CAPACITY: usize = 64;
const DEFAULT_FLOW_COUNT: usize = 100;
const IPCACHE_SYNC_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(15);

struct HubbleObserver {
    node_name: String,
    flow_tx: broadcast::Sender<Arc<Flow>>,
    flow_store: Arc<FlowStore>,
    agent_event_tx: broadcast::Sender<Arc<AgentEvent>>,
    agent_event_store: Arc<AgentEventStore>,
}

/// Check if a flow's timestamp falls within `since..until`.
fn flow_in_time_range(flow: &Flow, since: Option<&Timestamp>, until: Option<&Timestamp>) -> bool {
    let Some(flow_ts) = flow.time.as_ref() else {
        return true;
    };
    if let Some(s) = since
        && (flow_ts.seconds, flow_ts.nanos) < (s.seconds, s.nanos)
    {
        return false;
    }
    if let Some(u) = until
        && (flow_ts.seconds, flow_ts.nanos) > (u.seconds, u.nanos)
    {
        return false;
    }
    true
}

/// Check if a flow's timestamp is past the `until` bound.
fn flow_past_until(flow: &Flow, until: Option<&Timestamp>) -> bool {
    let Some(u) = until else {
        return false;
    };
    let Some(flow_ts) = flow.time.as_ref() else {
        return false;
    };
    (flow_ts.seconds, flow_ts.nanos) > (u.seconds, u.nanos)
}

#[tonic::async_trait]
impl Observer for HubbleObserver {
    type GetFlowsStream = ReceiverStream<Result<GetFlowsResponse, Status>>;
    type GetAgentEventsStream = ReceiverStream<Result<GetAgentEventsResponse, Status>>;
    type GetDebugEventsStream = ReceiverStream<Result<GetDebugEventsResponse, Status>>;

    async fn get_flows(
        &self,
        request: Request<GetFlowsRequest>,
    ) -> Result<Response<Self::GetFlowsStream>, Status> {
        let req = request.into_inner();
        let (tx, rx) = tokio::sync::mpsc::channel(GRPC_CHANNEL_CAPACITY);

        let filter = FlowFilterSet::compile(&req.whitelist, &req.blacklist);
        let node_name = self.node_name.clone();

        if req.follow {
            // Live streaming mode.
            let mut broadcast_rx = self.flow_tx.subscribe();
            let until = req.until;
            tokio::spawn(async move {
                loop {
                    match broadcast_rx.recv().await {
                        Ok(flow) => {
                            // Stop streaming if past the `until` bound.
                            if flow_past_until(&flow, until.as_ref()) {
                                break;
                            }
                            if !filter.is_empty() && !filter.matches(&flow) {
                                continue;
                            }
                            let resp = GetFlowsResponse {
                                response_types: Some(
                                    retina_proto::observer::get_flows_response::ResponseTypes::Flow(
                                        (*flow).clone(),
                                    ),
                                ),
                                node_name: node_name.clone(),
                                time: flow.time,
                            };
                            if tx.send(Ok(resp)).await.is_err() {
                                break; // Client disconnected.
                            }
                        }
                        Err(broadcast::error::RecvError::Lagged(n)) => {
                            tracing::warn!("flow subscriber lagged by {} events", n);
                        }
                        Err(broadcast::error::RecvError::Closed) => break,
                    }
                }
            });
        } else {
            // Historical mode.
            let n = if req.number > 0 {
                req.number as usize
            } else {
                DEFAULT_FLOW_COUNT
            };
            let flows = if req.first {
                self.flow_store.first_n(n)
            } else {
                self.flow_store.last_n(n)
            };
            let since = req.since;
            let until = req.until;
            tokio::spawn(async move {
                for flow in flows {
                    if !flow_in_time_range(&flow, since.as_ref(), until.as_ref()) {
                        continue;
                    }
                    if !filter.is_empty() && !filter.matches(&flow) {
                        continue;
                    }
                    let resp = GetFlowsResponse {
                        response_types: Some(
                            retina_proto::observer::get_flows_response::ResponseTypes::Flow(
                                (*flow).clone(),
                            ),
                        ),
                        node_name: node_name.clone(),
                        time: flow.time,
                    };
                    if tx.send(Ok(resp)).await.is_err() {
                        break;
                    }
                }
            });
        }

        Ok(Response::new(ReceiverStream::new(rx)))
    }

    async fn get_agent_events(
        &self,
        request: Request<GetAgentEventsRequest>,
    ) -> Result<Response<Self::GetAgentEventsStream>, Status> {
        let req = request.into_inner();
        let (tx, rx) = tokio::sync::mpsc::channel(GRPC_CHANNEL_CAPACITY);
        let node_name = self.node_name.clone();

        if req.follow {
            let mut broadcast_rx = self.agent_event_tx.subscribe();
            tokio::spawn(async move {
                loop {
                    match broadcast_rx.recv().await {
                        Ok(event) => {
                            let resp = GetAgentEventsResponse {
                                agent_event: Some((*event).clone()),
                                node_name: node_name.clone(),
                                time: None,
                            };
                            if tx.send(Ok(resp)).await.is_err() {
                                break;
                            }
                        }
                        Err(broadcast::error::RecvError::Lagged(n)) => {
                            tracing::warn!("agent event subscriber lagged by {} events", n);
                        }
                        Err(broadcast::error::RecvError::Closed) => break,
                    }
                }
            });
        } else {
            let n = if req.number > 0 {
                req.number as usize
            } else {
                DEFAULT_FLOW_COUNT
            };
            let events = if req.first {
                self.agent_event_store.first_n(n)
            } else {
                self.agent_event_store.last_n(n)
            };
            tokio::spawn(async move {
                for event in events {
                    let resp = GetAgentEventsResponse {
                        agent_event: Some((*event).clone()),
                        node_name: node_name.clone(),
                        time: None,
                    };
                    if tx.send(Ok(resp)).await.is_err() {
                        break;
                    }
                }
            });
        }

        Ok(Response::new(ReceiverStream::new(rx)))
    }

    async fn get_debug_events(
        &self,
        _request: Request<GetDebugEventsRequest>,
    ) -> Result<Response<Self::GetDebugEventsStream>, Status> {
        // No eBPF debug events exist yet — return an empty stream rather than
        // Unimplemented so `hubble observe --debug-events` doesn't error out.
        let (_tx, rx) = tokio::sync::mpsc::channel(1);
        Ok(Response::new(ReceiverStream::new(rx)))
    }

    async fn get_nodes(
        &self,
        _request: Request<GetNodesRequest>,
    ) -> Result<Response<GetNodesResponse>, Status> {
        let node = Node {
            name: self.node_name.clone(),
            version: env!("CARGO_PKG_VERSION").to_string(),
            address: String::new(),
            state: NodeState::NodeConnected.into(),
            tls: Some(Tls {
                enabled: false,
                server_name: String::new(),
            }),
            uptime_ns: self.flow_store.uptime_ns(),
            num_flows: self.flow_store.num_flows(),
            max_flows: self.flow_store.capacity() as u64,
            seen_flows: self.flow_store.seen_flows(),
        };
        Ok(Response::new(GetNodesResponse { nodes: vec![node] }))
    }

    async fn get_namespaces(
        &self,
        _request: Request<GetNamespacesRequest>,
    ) -> Result<Response<GetNamespacesResponse>, Status> {
        let flows = self.flow_store.all_flows();
        let mut namespaces = BTreeSet::new();
        for flow in &flows {
            if let Some(ref src) = flow.source
                && !src.namespace.is_empty()
            {
                namespaces.insert(src.namespace.clone());
            }
            if let Some(ref dst) = flow.destination
                && !dst.namespace.is_empty()
            {
                namespaces.insert(dst.namespace.clone());
            }
        }
        let ns_list: Vec<Namespace> = namespaces
            .into_iter()
            .map(|ns| Namespace {
                cluster: String::new(),
                namespace: ns,
            })
            .collect();
        Ok(Response::new(GetNamespacesResponse {
            namespaces: ns_list,
        }))
    }

    async fn server_status(
        &self,
        _request: Request<ServerStatusRequest>,
    ) -> Result<Response<ServerStatusResponse>, Status> {
        Ok(Response::new(ServerStatusResponse {
            num_flows: self.flow_store.num_flows(),
            max_flows: self.flow_store.capacity() as u64,
            seen_flows: self.flow_store.seen_flows(),
            uptime_ns: self.flow_store.uptime_ns(),
            num_connected_nodes: Some(1),
            num_unavailable_nodes: Some(0),
            unavailable_nodes: vec![],
            version: env!("CARGO_PKG_VERSION").to_string(),
            flows_rate: self.flow_store.flows_rate(),
        }))
    }
}

struct HubblePeer {
    grpc_port: u16,
    ip_cache: Arc<IpCache>,
}

#[tonic::async_trait]
impl Peer for HubblePeer {
    type NotifyStream = ReceiverStream<Result<ChangeNotification, Status>>;

    async fn notify(
        &self,
        _request: Request<NotifyRequest>,
    ) -> Result<Response<Self::NotifyStream>, Status> {
        let (tx, rx) = tokio::sync::mpsc::channel(PEER_CHANNEL_CAPACITY);
        let ip_cache = Arc::clone(&self.ip_cache);
        let grpc_port = self.grpc_port;

        tokio::spawn(async move {
            // Wait for the ipcache to sync with the operator so we have the
            // full node list before reporting peers.
            ip_cache.wait_synced(IPCACHE_SYNC_TIMEOUT).await;

            // Report all known nodes as peers.
            let nodes = ip_cache.get_node_peers();
            for (name, ip) in &nodes {
                let notification = ChangeNotification {
                    name: name.clone(),
                    address: format!("{}:{}", ip, grpc_port),
                    r#type: ChangeNotificationType::PeerAdded.into(),
                    tls: None,
                };
                if tx.send(Ok(notification)).await.is_err() {
                    return;
                }
            }

            // Subscribe to ipcache changes for reactive peer notifications.
            let mut known: std::collections::HashMap<String, std::net::IpAddr> =
                nodes.into_iter().collect();
            let mut events = ip_cache.subscribe();
            loop {
                match events.recv().await {
                    Ok(IpCacheEvent::Upsert(ip, identity)) => {
                        if identity.node_name.is_empty() {
                            continue;
                        }
                        let name: &str = &identity.node_name;
                        let change_type = match known.get(name) {
                            Some(old_ip) if *old_ip == ip => continue,
                            Some(_) => ChangeNotificationType::PeerUpdated,
                            None => ChangeNotificationType::PeerAdded,
                        };
                        known.insert(name.to_string(), ip);
                        let notification = ChangeNotification {
                            name: name.to_string(),
                            address: format!("{}:{}", ip, grpc_port),
                            r#type: change_type.into(),
                            tls: None,
                        };
                        if tx.send(Ok(notification)).await.is_err() {
                            return;
                        }
                    }
                    Ok(IpCacheEvent::Delete(ip)) => {
                        // Find and remove the peer with this IP.
                        let name = known
                            .iter()
                            .find(|(_, v)| **v == ip)
                            .map(|(k, _)| k.clone());
                        if let Some(name) = name {
                            known.remove(&name);
                            let notification = ChangeNotification {
                                name,
                                address: String::new(),
                                r#type: ChangeNotificationType::PeerDeleted.into(),
                                tls: None,
                            };
                            if tx.send(Ok(notification)).await.is_err() {
                                return;
                            }
                        }
                    }
                    Ok(IpCacheEvent::Clear) => {
                        // Cache was reset (operator reconnect) — remove all known peers.
                        for name in known.keys() {
                            let notification = ChangeNotification {
                                name: name.clone(),
                                address: String::new(),
                                r#type: ChangeNotificationType::PeerDeleted.into(),
                                tls: None,
                            };
                            if tx.send(Ok(notification)).await.is_err() {
                                return;
                            }
                        }
                        known.clear();
                    }
                    Err(broadcast::error::RecvError::Lagged(n)) => {
                        tracing::warn!("peer subscriber lagged by {} events, reconciling", n);
                        // After lag, reconcile by diffing known state against current cache.
                        let current: std::collections::HashMap<String, std::net::IpAddr> =
                            ip_cache.get_node_peers().into_iter().collect();
                        for name in known.keys().filter(|n| !current.contains_key(*n)) {
                            let notification = ChangeNotification {
                                name: name.clone(),
                                address: String::new(),
                                r#type: ChangeNotificationType::PeerDeleted.into(),
                                tls: None,
                            };
                            if tx.send(Ok(notification)).await.is_err() {
                                return;
                            }
                        }
                        for (name, ip) in &current {
                            let change_type = match known.get(name) {
                                Some(old_ip) if *old_ip == *ip => continue,
                                Some(_) => ChangeNotificationType::PeerUpdated,
                                None => ChangeNotificationType::PeerAdded,
                            };
                            let notification = ChangeNotification {
                                name: name.clone(),
                                address: format!("{}:{}", ip, grpc_port),
                                r#type: change_type.into(),
                                tls: None,
                            };
                            if tx.send(Ok(notification)).await.is_err() {
                                return;
                            }
                        }
                        known = current;
                    }
                    Err(broadcast::error::RecvError::Closed) => return,
                }
            }
        });

        Ok(Response::new(ReceiverStream::new(rx)))
    }
}

/// Start the Hubble Observer gRPC server.
#[allow(clippy::too_many_arguments)]
pub async fn serve(
    port: u16,
    node_name: String,
    flow_tx: broadcast::Sender<Arc<Flow>>,
    flow_store: Arc<FlowStore>,
    agent_event_tx: broadcast::Sender<Arc<AgentEvent>>,
    agent_event_store: Arc<AgentEventStore>,
    agent_state: Arc<AgentState>,
    ip_cache: Arc<IpCache>,
) -> anyhow::Result<()> {
    let addr = format!("0.0.0.0:{}", port).parse()?;
    let observer = HubbleObserver {
        node_name,
        flow_tx,
        flow_store,
        agent_event_tx,
        agent_event_store,
    };
    let peer = HubblePeer {
        grpc_port: port,
        ip_cache,
    };

    info!(%addr, "starting Hubble Observer gRPC server");

    agent_state.grpc_bound.store(true, Ordering::Release);

    tonic::transport::Server::builder()
        .add_service(
            ObserverServer::new(observer)
                .accept_compressed(CompressionEncoding::Gzip)
                .send_compressed(CompressionEncoding::Gzip),
        )
        .add_service(
            PeerServer::new(peer)
                .accept_compressed(CompressionEncoding::Gzip)
                .send_compressed(CompressionEncoding::Gzip),
        )
        .serve(addr)
        .await?;

    Ok(())
}
