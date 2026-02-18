use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use anyhow::{Context as _, Result};
use aya::programs::tc::SchedClassifierLink;
use aya::Ebpf;
use futures::stream::{StreamExt, TryStreamExt};
use netlink_packet_core::NetlinkPayload;
use netlink_packet_route::link::{InfoKind, LinkAttribute, LinkInfo, LinkMessage};
use netlink_packet_route::RouteNetlinkMessage;
use netlink_sys::{AsyncSocket, SocketAddr};
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
    attached: HashMap<u32, EndpointAttachment>, // ifindex → attachment
}

impl VethWatcher {
    pub fn new(ebpf: Arc<Mutex<Ebpf>>) -> Self {
        Self {
            ebpf,
            attached: HashMap::new(),
        }
    }

    pub async fn run(&mut self) -> Result<()> {
        let (mut connection, handle, mut messages) = rtnetlink::new_connection()
            .context("failed to create netlink connection")?;

        // Subscribe to link events before spawning the connection.
        let addr = SocketAddr::new(0, RTMGRP_LINK);
        connection
            .socket_mut()
            .socket_mut()
            .bind(&addr)
            .context("failed to bind netlink socket to RTMGRP_LINK")?;

        tokio::spawn(connection);

        info!("watching for veth interfaces...");

        // Dump existing links to catch veths that already exist.
        let mut links = handle.link().get().execute();
        while let Some(msg) = links.try_next().await.context("failed to dump links")? {
            if let Some((ifindex, ifname)) = parse_veth_msg(&msg) {
                self.attach(ifindex, ifname);
            }
        }

        // Event loop for new/deleted links.
        while let Some((message, _)) = messages.next().await {
            if let NetlinkPayload::InnerMessage(inner) = message.payload {
                match inner {
                    RouteNetlinkMessage::NewLink(msg) => {
                        if let Some((ifindex, ifname)) = parse_veth_msg(&msg) {
                            self.attach(ifindex, ifname);
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

    fn attach(&mut self, ifindex: u32, ifname: String) {
        if self.attached.contains_key(&ifindex) {
            return;
        }

        let mut ebpf = self.ebpf.lock().unwrap();
        match loader::attach_endpoint(&mut ebpf, &ifname) {
            Ok((ingress, egress)) => {
                info!(ifindex, ifname = %ifname, "attached endpoint programs to veth");
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

/// Extract (ifindex, ifname) from a link message if it's a veth.
fn parse_veth_msg(msg: &LinkMessage) -> Option<(u32, String)> {
    let ifindex = msg.header.index;
    let mut ifname = None;
    let mut is_veth = false;

    for attr in &msg.attributes {
        match attr {
            LinkAttribute::IfName(name) => ifname = Some(name.clone()),
            LinkAttribute::LinkInfo(infos) => {
                for info in infos {
                    if let LinkInfo::Kind(InfoKind::Veth) = info {
                        is_veth = true;
                    }
                }
            }
            _ => {}
        }
    }

    if is_veth {
        // Only attach to interfaces with the standard pod-veth naming
        // convention (e.g. vethc38b9e28). In kind clusters, eth0 is also
        // a veth but carries node-level traffic we don't want here.
        ifname.filter(|name| name.starts_with("veth")).map(|name| (ifindex, name))
    } else {
        None
    }
}
