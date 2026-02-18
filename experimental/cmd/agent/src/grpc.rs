use std::sync::Arc;
use std::sync::atomic::Ordering;

use retina_core::metrics::AgentState;
use retina_core::store::FlowStore;
use retina_proto::{
    flow::Flow,
    observer::{
        observer_server::{Observer, ObserverServer},
        GetAgentEventsRequest, GetAgentEventsResponse, GetDebugEventsRequest,
        GetDebugEventsResponse, GetFlowsRequest, GetFlowsResponse, GetNamespacesRequest,
        GetNamespacesResponse, GetNodesRequest, GetNodesResponse, ServerStatusRequest,
        ServerStatusResponse,
    },
};
use tokio::sync::broadcast;
use tokio_stream::wrappers::ReceiverStream;
use tonic::{Request, Response, Status};
use tracing::info;

struct HubbleObserver {
    flow_tx: broadcast::Sender<Arc<Flow>>,
    flow_store: Arc<FlowStore>,
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
        let (tx, rx) = tokio::sync::mpsc::channel(256);

        if req.follow {
            // Live streaming mode.
            let mut broadcast_rx = self.flow_tx.subscribe();
            tokio::spawn(async move {
                loop {
                    match broadcast_rx.recv().await {
                        Ok(flow) => {
                            let resp = GetFlowsResponse {
                                response_types: Some(
                                    retina_proto::observer::get_flows_response::ResponseTypes::Flow(
                                        (*flow).clone(),
                                    ),
                                ),
                                node_name: String::new(),
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
            // Historical mode: return last N flows.
            let n = if req.number > 0 {
                req.number as usize
            } else {
                100
            };
            let flows = self.flow_store.last_n(n);
            let tx_clone = tx.clone();
            tokio::spawn(async move {
                for flow in flows {
                    let resp = GetFlowsResponse {
                        response_types: Some(
                            retina_proto::observer::get_flows_response::ResponseTypes::Flow(
                                (*flow).clone(),
                            ),
                        ),
                        node_name: String::new(),
                        time: flow.time,
                    };
                    if tx_clone.send(Ok(resp)).await.is_err() {
                        break;
                    }
                }
            });
        }

        Ok(Response::new(ReceiverStream::new(rx)))
    }

    async fn get_agent_events(
        &self,
        _request: Request<GetAgentEventsRequest>,
    ) -> Result<Response<Self::GetAgentEventsStream>, Status> {
        Err(Status::unimplemented("GetAgentEvents not implemented"))
    }

    async fn get_debug_events(
        &self,
        _request: Request<GetDebugEventsRequest>,
    ) -> Result<Response<Self::GetDebugEventsStream>, Status> {
        Err(Status::unimplemented("GetDebugEvents not implemented"))
    }

    async fn get_nodes(
        &self,
        _request: Request<GetNodesRequest>,
    ) -> Result<Response<GetNodesResponse>, Status> {
        Err(Status::unimplemented("GetNodes not implemented"))
    }

    async fn get_namespaces(
        &self,
        _request: Request<GetNamespacesRequest>,
    ) -> Result<Response<GetNamespacesResponse>, Status> {
        Err(Status::unimplemented("GetNamespaces not implemented"))
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
            ..Default::default()
        }))
    }
}

/// Start the Hubble Observer gRPC server.
pub async fn serve(
    port: u16,
    flow_tx: broadcast::Sender<Arc<Flow>>,
    flow_store: Arc<FlowStore>,
    agent_state: Arc<AgentState>,
) -> anyhow::Result<()> {
    let addr = format!("0.0.0.0:{}", port).parse()?;
    let observer = HubbleObserver {
        flow_tx,
        flow_store,
    };

    info!(%addr, "starting Hubble Observer gRPC server");

    agent_state.grpc_bound.store(true, Ordering::Release);

    tonic::transport::Server::builder()
        .add_service(ObserverServer::new(observer))
        .serve(addr)
        .await?;

    Ok(())
}
