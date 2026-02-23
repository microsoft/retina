use std::collections::HashMap;
use std::net::IpAddr;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use anyhow::{Context as _, Result};
use aya::Ebpf;
use aya::programs::tc::SchedClassifierLink;
use futures::stream::{StreamExt, TryStreamExt};
use netlink_packet_core::NetlinkPayload;
use netlink_packet_route::RouteNetlinkMessage;
use netlink_packet_route::link::{LinkAttribute, LinkMessage};
use netlink_packet_route::neighbour::NeighbourAttribute;
use netlink_sys::{AsyncSocket, SocketAddr};
use retina_core::ipcache::IpCache;
use rtnetlink::constants::RTMGRP_LINK;
use tracing::{info, warn};

use crate::loader;

struct EndpointAttachment {
    _ingress: SchedClassifierLink, // Drop auto-detaches
    _egress: SchedClassifierLink,
    ifname: String,
}

pub struct VethWatcher {
    ebpf: Arc<Mutex<Ebpf>>,
    ip_cache: Arc<IpCache>,
    attached: HashMap<u32, EndpointAttachment>, // ifindex → attachment
}

impl VethWatcher {
    pub fn new(ebpf: Arc<Mutex<Ebpf>>, ip_cache: Arc<IpCache>) -> Self {
        Self {
            ebpf,
            ip_cache,
            attached: HashMap::new(),
        }
    }

    pub async fn run(&mut self) -> Result<()> {
        let (mut connection, handle, mut messages) =
            rtnetlink::new_connection().context("failed to create netlink connection")?;

        // Subscribe to link events before spawning the connection.
        let addr = SocketAddr::new(0, RTMGRP_LINK);
        connection
            .socket_mut()
            .socket_mut()
            .bind(&addr)
            .context("failed to bind netlink socket to RTMGRP_LINK")?;

        tokio::spawn(connection);

        // Wait for the IP cache to complete its initial sync before attaching
        // programs, so we can resolve pod names for every veth and the enricher
        // has identity data from the first packet.
        if !self.ip_cache.wait_synced(Duration::from_secs(30)).await {
            warn!("IP cache sync timed out after 30s, proceeding without full cache");
        }

        info!("watching for veth interfaces...");

        // Dump existing links to catch veths that already exist.
        let mut links = handle.link().get().execute();
        while let Some(msg) = links.try_next().await.context("failed to dump links")? {
            if let Some((ifindex, ifname)) = parse_veth_msg(&msg) {
                self.attach(ifindex, ifname, &handle).await;
            }
        }

        // Event loop for new/deleted links.
        while let Some((message, _)) = messages.next().await {
            if let NetlinkPayload::InnerMessage(inner) = message.payload {
                match inner {
                    RouteNetlinkMessage::NewLink(msg) => {
                        if let Some((ifindex, ifname)) = parse_veth_msg(&msg) {
                            self.attach(ifindex, ifname, &handle).await;
                        }
                    }
                    RouteNetlinkMessage::DelLink(msg) => {
                        let ifindex = msg.header.index;
                        self.detach(ifindex);
                    }
                    _ => {}
                }
            }
        }

        Ok(())
    }

    async fn attach(&mut self, ifindex: u32, ifname: String, handle: &rtnetlink::Handle) {
        if self.attached.contains_key(&ifindex) {
            return;
        }

        // Scope the mutex guard so it's dropped before any .await.
        let result = {
            let mut ebpf = self.ebpf.lock().expect("lock poisoned");
            loader::attach_endpoint(&mut ebpf, &ifname)
        };

        match result {
            Ok((ingress, egress)) => {
                if let Some((ns, pod)) = resolve_pod_name(ifindex, handle, &self.ip_cache).await {
                    info!(ifindex, ifname = %ifname, pod = %format!("{ns}/{pod}"), "attached endpoint programs to veth");
                } else {
                    info!(ifindex, ifname = %ifname, "attached endpoint programs to veth");
                }

                self.attached.insert(
                    ifindex,
                    EndpointAttachment {
                        _ingress: ingress,
                        _egress: egress,
                        ifname,
                    },
                );
            }
            Err(e) => {
                warn!(ifindex, ifname = %ifname, "failed to attach endpoint programs: {e}");
            }
        }
    }

    fn detach(&mut self, ifindex: u32) {
        if let Some(attachment) = self.attached.remove(&ifindex) {
            info!(
                ifindex,
                ifname = %attachment.ifname,
                "detached endpoint programs from veth"
            );
        }
    }
}

/// Look up the pod name for a veth by querying the neighbor table for its IP,
/// then resolving the IP in the IP cache. Returns `Some((namespace, pod_name))`
/// on success, `None` if the neighbor entry or cache entry isn't found.
async fn resolve_pod_name(
    ifindex: u32,
    handle: &rtnetlink::Handle,
    ip_cache: &IpCache,
) -> Option<(String, String)> {
    let mut neighbours = handle.neighbours().get().execute();
    while let Some(Ok(neigh)) = neighbours.next().await {
        if neigh.header.ifindex != ifindex {
            continue;
        }
        for attr in &neigh.attributes {
            if let NeighbourAttribute::Destination(dest) = attr {
                let ip: IpAddr = match dest {
                    netlink_packet_route::neighbour::NeighbourAddress::Inet(v4) => (*v4).into(),
                    netlink_packet_route::neighbour::NeighbourAddress::Inet6(v6) => (*v6).into(),
                    _ => continue,
                };
                if let Some(identity) = ip_cache.get(&ip)
                    && !identity.pod_name.is_empty()
                {
                    return Some((
                        identity.namespace.to_string(),
                        identity.pod_name.to_string(),
                    ));
                }
            }
        }
    }
    None
}

/// Extract (ifindex, ifname) from a link message if it's a pod veth.
///
/// Detection is CNI-agnostic: any interface whose peer lives in a different
/// network namespace (indicated by the `NetNsId` netlink attribute) is a pod
/// veth. This works for all CNI implementations regardless of naming convention.
fn parse_veth_msg(msg: &LinkMessage) -> Option<(u32, String)> {
    let ifindex = msg.header.index;
    let mut ifname = None;
    let mut has_peer_netns = false;

    for attr in &msg.attributes {
        match attr {
            LinkAttribute::IfName(name) => ifname = Some(name.clone()),
            LinkAttribute::LinkNetNsId(_) => has_peer_netns = true,
            _ => {}
        }
    }

    if has_peer_netns {
        ifname.map(|name| (ifindex, name))
    } else {
        None
    }
}
