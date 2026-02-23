//! Shared test data factories for criterion benchmarks.
//!
//! Included via `mod bench_helpers;` from each benchmark file (not a bench target itself).

#![allow(dead_code, deprecated)]

use std::net::{IpAddr, Ipv4Addr};
use std::sync::Arc;

use retina_common::{
    PacketEvent, DIR_EGRESS, DIR_INGRESS, IPPROTO_TCP, IPPROTO_UDP, OBS_FROM_ENDPOINT,
    OBS_TO_ENDPOINT, TCP_ACK, TCP_SYN,
};
use retina_core::ipcache::{Identity, IpCache, Workload};
use retina_proto::flow::{self, Flow, FlowFilter, IpVersion, TcpFlags, TrafficDirection, Verdict};

// ── PacketEvent factories ──────────────────────────────────────────

/// TCP SYN packet: 10.0.0.1:12345 → 10.0.0.2:80
pub fn make_tcp_syn_event() -> PacketEvent {
    PacketEvent {
        ts_ns: 1_700_000_000_000_000_000,
        bytes: 64,
        src_ip: u32::from(Ipv4Addr::new(10, 0, 0, 1)),
        dst_ip: u32::from(Ipv4Addr::new(10, 0, 0, 2)),
        src_port: 12345,
        dst_port: 80,
        proto: IPPROTO_TCP,
        observation_point: OBS_TO_ENDPOINT,
        traffic_direction: DIR_INGRESS,
        flags: TCP_SYN,
        is_reply: 0,
        ..Default::default()
    }
}

/// TCP SYN-ACK reply: 10.0.0.2:80 → 10.0.0.1:12345
pub fn make_tcp_syn_ack_event() -> PacketEvent {
    PacketEvent {
        ts_ns: 1_700_000_000_000_000_000,
        bytes: 64,
        src_ip: u32::from(Ipv4Addr::new(10, 0, 0, 2)),
        dst_ip: u32::from(Ipv4Addr::new(10, 0, 0, 1)),
        src_port: 80,
        dst_port: 12345,
        proto: IPPROTO_TCP,
        observation_point: OBS_FROM_ENDPOINT,
        traffic_direction: DIR_EGRESS,
        flags: TCP_SYN | TCP_ACK,
        is_reply: 1,
        ..Default::default()
    }
}

/// TCP ACK data packet with 1500 bytes.
pub fn make_tcp_ack_event() -> PacketEvent {
    PacketEvent {
        ts_ns: 1_700_000_000_000_000_000,
        bytes: 1500,
        src_ip: u32::from(Ipv4Addr::new(10, 0, 0, 1)),
        dst_ip: u32::from(Ipv4Addr::new(10, 0, 0, 2)),
        src_port: 12345,
        dst_port: 80,
        proto: IPPROTO_TCP,
        observation_point: OBS_TO_ENDPOINT,
        traffic_direction: DIR_INGRESS,
        flags: TCP_ACK,
        is_reply: 0,
        ..Default::default()
    }
}

/// UDP packet (DNS-like): 10.0.0.1:45678 → 10.0.0.2:53
pub fn make_udp_event() -> PacketEvent {
    PacketEvent {
        ts_ns: 1_700_000_000_000_000_000,
        bytes: 128,
        src_ip: u32::from(Ipv4Addr::new(10, 0, 0, 1)),
        dst_ip: u32::from(Ipv4Addr::new(10, 0, 0, 2)),
        src_port: 45678,
        dst_port: 53,
        proto: IPPROTO_UDP,
        observation_point: OBS_FROM_ENDPOINT,
        traffic_direction: DIR_EGRESS,
        flags: 0,
        is_reply: 0,
        ..Default::default()
    }
}

// ── Identity factories ─────────────────────────────────────────────

pub fn make_pod_identity(namespace: &str, pod_name: &str, labels: &[&str]) -> Identity {
    Identity {
        namespace: Arc::from(namespace),
        pod_name: Arc::from(pod_name),
        service_name: Arc::from(""),
        node_name: Arc::from(""),
        labels: labels
            .iter()
            .map(|l| Arc::from(*l))
            .collect::<Vec<_>>()
            .into(),
        workloads: vec![Workload {
            name: Arc::from(pod_name.split('-').next().unwrap_or(pod_name)),
            kind: Arc::from("Deployment"),
        }]
        .into(),
    }
}

pub fn make_node_identity(node_name: &str) -> Identity {
    Identity {
        namespace: Arc::from(""),
        pod_name: Arc::from(""),
        service_name: Arc::from(""),
        node_name: Arc::from(node_name),
        labels: Arc::from(Vec::<Arc<str>>::new()),
        workloads: Arc::from(Vec::<Workload>::new()),
    }
}

pub fn make_service_identity(namespace: &str, svc_name: &str) -> Identity {
    Identity {
        namespace: Arc::from(namespace),
        pod_name: Arc::from(""),
        service_name: Arc::from(svc_name),
        node_name: Arc::from(""),
        labels: Arc::from(Vec::<Arc<str>>::new()),
        workloads: Arc::from(Vec::<Workload>::new()),
    }
}

// ── IpCache factories ──────────────────────────────────────────────

/// Create an IpCache with `n` pod entries (10.0.x.y addresses).
/// Each pod has 3-5 labels to simulate realistic conditions.
pub fn make_populated_ipcache(n: usize) -> IpCache {
    let cache = IpCache::new();
    cache.set_local_node_name("bench-node".into());
    for i in 0..n {
        let a = ((i >> 8) & 0xFF) as u8;
        let b = (i & 0xFF) as u8;
        let ip = IpAddr::V4(Ipv4Addr::new(10, 0, a, b));
        let ns = match i % 4 {
            0 => "default",
            1 => "kube-system",
            2 => "backend",
            _ => "monitoring",
        };
        let labels: &[&str] = match i % 3 {
            0 => &["app=nginx", "tier=frontend", "env=prod"],
            1 => &["app=redis", "tier=backend", "env=prod", "version=v2"],
            _ => &[
                "app=coredns",
                "k8s-app=kube-dns",
                "tier=infra",
                "env=prod",
                "managed-by=helm",
            ],
        };
        cache.upsert(ip, make_pod_identity(ns, &format!("pod-{i}"), labels));
    }
    cache.mark_synced();
    cache
}

// ── Flow factories ─────────────────────────────────────────────────

/// Flow with no enrichment (raw from packet_event_to_flow).
pub fn make_raw_flow() -> Flow {
    retina_core::flow::packet_event_to_flow(&make_tcp_syn_event(), 0)
}

/// Fully enriched flow with both endpoints populated.
pub fn make_enriched_flow() -> Flow {
    Flow {
        ip: Some(flow::Ip {
            source: "10.0.0.1".to_string(),
            destination: "10.0.0.2".to_string(),
            ip_version: IpVersion::IPv4.into(),
            ..Default::default()
        }),
        l4: Some(flow::Layer4 {
            protocol: Some(flow::layer4::Protocol::Tcp(flow::Tcp {
                source_port: 12345,
                destination_port: 80,
                flags: Some(TcpFlags {
                    syn: true,
                    ack: true,
                    ..Default::default()
                }),
            })),
        }),
        source: Some(flow::Endpoint {
            namespace: "default".to_string(),
            pod_name: "web-abc123".to_string(),
            labels: vec!["app=web".to_string(), "env=prod".to_string()],
            identity: 42000,
            workloads: vec![flow::Workload {
                name: "web".to_string(),
                kind: "Deployment".to_string(),
            }],
            ..Default::default()
        }),
        destination: Some(flow::Endpoint {
            namespace: "kube-system".to_string(),
            pod_name: "coredns-xyz".to_string(),
            labels: vec!["k8s-app=kube-dns".to_string()],
            identity: 42001,
            workloads: vec![flow::Workload {
                name: "coredns".to_string(),
                kind: "Deployment".to_string(),
            }],
            ..Default::default()
        }),
        verdict: Verdict::Forwarded.into(),
        traffic_direction: TrafficDirection::Egress.into(),
        is_reply: Some(false),
        node_name: "bench-node".to_string(),
        summary: "TCP Flags: SYN-ACK".to_string(),
        ..Default::default()
    }
}

// ── Filter factories ───────────────────────────────────────────────

/// No filters (pass-through).
pub fn empty_filters() -> (Vec<FlowFilter>, Vec<FlowFilter>) {
    (vec![], vec![])
}

/// Simple whitelist: single source IP match.
pub fn simple_whitelist() -> (Vec<FlowFilter>, Vec<FlowFilter>) {
    (
        vec![FlowFilter {
            source_ip: vec!["10.0.0.1".to_string()],
            ..Default::default()
        }],
        vec![],
    )
}

/// Complex filter: whitelist with CIDR + protocol + port, blacklist with pod name.
pub fn complex_filters() -> (Vec<FlowFilter>, Vec<FlowFilter>) {
    (
        vec![
            FlowFilter {
                source_ip: vec!["10.0.0.0/16".to_string()],
                protocol: vec!["TCP".to_string()],
                destination_port: vec!["80".to_string(), "443".to_string()],
                ..Default::default()
            },
            FlowFilter {
                source_label: vec!["app=web".to_string()],
                traffic_direction: vec![TrafficDirection::Egress.into()],
                ..Default::default()
            },
        ],
        vec![FlowFilter {
            destination_pod: vec!["kube-system/".to_string()],
            ..Default::default()
        }],
    )
}
