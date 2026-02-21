use std::net::IpAddr;

use retina_proto::flow::{self, Flow, FlowFilter, TcpFlags};

/// Compiled filter set for matching flows against whitelist/blacklist rules.
///
/// Hubble semantics:
/// - Within one FlowFilter: all non-empty fields must match (AND).
///   Each repeated field is OR across its elements.
/// - Whitelist: OR across FlowFilters (any match includes).
/// - Blacklist: OR across FlowFilters (any match excludes).
/// - Result: `(whitelist_empty OR matches_whitelist) AND NOT matches_blacklist`.
pub struct FlowFilterSet {
    whitelist: Vec<CompiledFilter>,
    blacklist: Vec<CompiledFilter>,
}

impl FlowFilterSet {
    /// Compile proto FlowFilter lists into an efficient filter set.
    pub fn compile(whitelist: &[FlowFilter], blacklist: &[FlowFilter]) -> Self {
        Self {
            whitelist: whitelist.iter().map(CompiledFilter::from_proto).collect(),
            blacklist: blacklist.iter().map(CompiledFilter::from_proto).collect(),
        }
    }

    /// Returns true if no filters are configured (pass everything through).
    pub fn is_empty(&self) -> bool {
        self.whitelist.is_empty() && self.blacklist.is_empty()
    }

    /// Returns true if the flow matches the filter set.
    pub fn matches(&self, flow: &Flow) -> bool {
        let wl_ok =
            self.whitelist.is_empty() || self.whitelist.iter().any(|f| f.matches(flow));
        let bl_ok = !self.blacklist.iter().any(|f| f.matches(flow));
        wl_ok && bl_ok
    }
}

// ── Compiled individual filter ──────────────────────────────────────────────

struct CompiledFilter {
    source_ip: Vec<IpMatcher>,
    destination_ip: Vec<IpMatcher>,
    source_pod: Vec<PodMatcher>,
    destination_pod: Vec<PodMatcher>,
    source_label: Vec<String>,
    destination_label: Vec<String>,
    verdict: Vec<i32>,
    traffic_direction: Vec<i32>,
    protocol: Vec<String>,
    source_port: Vec<u32>,
    destination_port: Vec<u32>,
    tcp_flags: Vec<TcpFlags>,
    reply: Vec<bool>,
    ip_version: Vec<i32>,
    node_name: Vec<NodeNameMatcher>,
    source_identity: Vec<u32>,
    destination_identity: Vec<u32>,
}

impl CompiledFilter {
    fn from_proto(f: &FlowFilter) -> Self {
        Self {
            source_ip: f.source_ip.iter().filter_map(|s| IpMatcher::parse(s)).collect(),
            destination_ip: f.destination_ip.iter().filter_map(|s| IpMatcher::parse(s)).collect(),
            source_pod: f.source_pod.iter().map(|s| PodMatcher::parse(s)).collect(),
            destination_pod: f.destination_pod.iter().map(|s| PodMatcher::parse(s)).collect(),
            source_label: f.source_label.clone(),
            destination_label: f.destination_label.clone(),
            verdict: f.verdict.clone(),
            traffic_direction: f.traffic_direction.clone(),
            protocol: f.protocol.iter().map(|s| s.to_lowercase()).collect(),
            source_port: f
                .source_port
                .iter()
                .filter_map(|s| s.parse::<u32>().ok())
                .collect(),
            destination_port: f
                .destination_port
                .iter()
                .filter_map(|s| s.parse::<u32>().ok())
                .collect(),
            tcp_flags: f.tcp_flags.clone(),
            reply: f.reply.clone(),
            ip_version: f.ip_version.clone(),
            node_name: f.node_name.iter().map(|s| NodeNameMatcher::parse(s)).collect(),
            source_identity: f.source_identity.clone(),
            destination_identity: f.destination_identity.clone(),
        }
    }

    /// All non-empty fields must match (AND). Each field is OR across elements.
    fn matches(&self, flow: &Flow) -> bool {
        self.match_source_ip(flow)
            && self.match_destination_ip(flow)
            && self.match_source_pod(flow)
            && self.match_destination_pod(flow)
            && self.match_source_label(flow)
            && self.match_destination_label(flow)
            && self.match_verdict(flow)
            && self.match_traffic_direction(flow)
            && self.match_protocol(flow)
            && self.match_source_port(flow)
            && self.match_destination_port(flow)
            && self.match_tcp_flags(flow)
            && self.match_reply(flow)
            && self.match_ip_version(flow)
            && self.match_node_name(flow)
            && self.match_source_identity(flow)
            && self.match_destination_identity(flow)
    }

    fn match_source_ip(&self, flow: &Flow) -> bool {
        if self.source_ip.is_empty() {
            return true;
        }
        let ip_str = match flow.ip.as_ref() {
            Some(ip) => &ip.source,
            None => return false,
        };
        let ip: IpAddr = match ip_str.parse() {
            Ok(ip) => ip,
            Err(_) => return false,
        };
        self.source_ip.iter().any(|m| m.matches(ip))
    }

    fn match_destination_ip(&self, flow: &Flow) -> bool {
        if self.destination_ip.is_empty() {
            return true;
        }
        let ip_str = match flow.ip.as_ref() {
            Some(ip) => &ip.destination,
            None => return false,
        };
        let ip: IpAddr = match ip_str.parse() {
            Ok(ip) => ip,
            Err(_) => return false,
        };
        self.destination_ip.iter().any(|m| m.matches(ip))
    }

    fn match_source_pod(&self, flow: &Flow) -> bool {
        if self.source_pod.is_empty() {
            return true;
        }
        let ep = match flow.source.as_ref() {
            Some(ep) => ep,
            None => return false,
        };
        self.source_pod.iter().any(|m| m.matches(&ep.namespace, &ep.pod_name))
    }

    fn match_destination_pod(&self, flow: &Flow) -> bool {
        if self.destination_pod.is_empty() {
            return true;
        }
        let ep = match flow.destination.as_ref() {
            Some(ep) => ep,
            None => return false,
        };
        self.destination_pod.iter().any(|m| m.matches(&ep.namespace, &ep.pod_name))
    }

    fn match_source_label(&self, flow: &Flow) -> bool {
        if self.source_label.is_empty() {
            return true;
        }
        let ep = match flow.source.as_ref() {
            Some(ep) => ep,
            None => return false,
        };
        self.source_label
            .iter()
            .all(|label| ep.labels.iter().any(|l| l == label))
    }

    fn match_destination_label(&self, flow: &Flow) -> bool {
        if self.destination_label.is_empty() {
            return true;
        }
        let ep = match flow.destination.as_ref() {
            Some(ep) => ep,
            None => return false,
        };
        self.destination_label
            .iter()
            .all(|label| ep.labels.iter().any(|l| l == label))
    }

    fn match_verdict(&self, flow: &Flow) -> bool {
        if self.verdict.is_empty() {
            return true;
        }
        self.verdict.iter().any(|v| *v == flow.verdict)
    }

    fn match_traffic_direction(&self, flow: &Flow) -> bool {
        if self.traffic_direction.is_empty() {
            return true;
        }
        self.traffic_direction
            .iter()
            .any(|d| *d == flow.traffic_direction)
    }

    fn match_protocol(&self, flow: &Flow) -> bool {
        if self.protocol.is_empty() {
            return true;
        }
        let l4 = match flow.l4.as_ref().and_then(|l4| l4.protocol.as_ref()) {
            Some(p) => p,
            None => return false,
        };
        let flow_proto = match l4 {
            flow::layer4::Protocol::Tcp(_) => "tcp",
            flow::layer4::Protocol::Udp(_) => "udp",
            flow::layer4::Protocol::IcmPv4(_) => "icmpv4",
            flow::layer4::Protocol::IcmPv6(_) => "icmpv6",
            flow::layer4::Protocol::Sctp(_) => "sctp",
            flow::layer4::Protocol::Vrrp(_) => "vrrp",
            flow::layer4::Protocol::Igmp(_) => "igmp",
        };
        self.protocol.iter().any(|p| p == flow_proto)
    }

    fn match_source_port(&self, flow: &Flow) -> bool {
        if self.source_port.is_empty() {
            return true;
        }
        let port = extract_source_port(flow);
        match port {
            Some(p) => self.source_port.iter().any(|sp| *sp == p),
            None => false,
        }
    }

    fn match_destination_port(&self, flow: &Flow) -> bool {
        if self.destination_port.is_empty() {
            return true;
        }
        let port = extract_destination_port(flow);
        match port {
            Some(p) => self.destination_port.iter().any(|dp| *dp == p),
            None => false,
        }
    }

    fn match_tcp_flags(&self, flow: &Flow) -> bool {
        if self.tcp_flags.is_empty() {
            return true;
        }
        let flow_flags = match flow
            .l4
            .as_ref()
            .and_then(|l4| l4.protocol.as_ref())
        {
            Some(flow::layer4::Protocol::Tcp(tcp)) => match tcp.flags.as_ref() {
                Some(f) => f,
                None => return false,
            },
            _ => return false,
        };
        // Any of the filter flag sets must be a subset of the flow's flags.
        self.tcp_flags.iter().any(|f| tcp_flags_subset(f, flow_flags))
    }

    fn match_reply(&self, flow: &Flow) -> bool {
        if self.reply.is_empty() {
            return true;
        }
        let is_reply = match flow.is_reply {
            Some(v) => v,
            None => return false,
        };
        self.reply.iter().any(|r| *r == is_reply)
    }

    fn match_ip_version(&self, flow: &Flow) -> bool {
        if self.ip_version.is_empty() {
            return true;
        }
        let ver = match flow.ip.as_ref() {
            Some(ip) => ip.ip_version,
            None => return false,
        };
        self.ip_version.iter().any(|v| *v == ver)
    }

    fn match_node_name(&self, flow: &Flow) -> bool {
        if self.node_name.is_empty() {
            return true;
        }
        self.node_name.iter().any(|m| m.matches(&flow.node_name))
    }

    fn match_source_identity(&self, flow: &Flow) -> bool {
        if self.source_identity.is_empty() {
            return true;
        }
        let id = match flow.source.as_ref() {
            Some(ep) => ep.identity,
            None => return false,
        };
        self.source_identity.iter().any(|i| *i == id)
    }

    fn match_destination_identity(&self, flow: &Flow) -> bool {
        if self.destination_identity.is_empty() {
            return true;
        }
        let id = match flow.destination.as_ref() {
            Some(ep) => ep.identity,
            None => return false,
        };
        self.destination_identity.iter().any(|i| *i == id)
    }
}

// ── Helper types ────────────────────────────────────────────────────────────

enum IpMatcher {
    Exact(IpAddr),
    Cidr(IpAddr, u8),
}

impl IpMatcher {
    fn parse(s: &str) -> Option<Self> {
        if let Some((addr_str, prefix_str)) = s.split_once('/') {
            let addr: IpAddr = addr_str.parse().ok()?;
            let prefix: u8 = prefix_str.parse().ok()?;
            Some(IpMatcher::Cidr(addr, prefix))
        } else {
            let addr: IpAddr = s.parse().ok()?;
            Some(IpMatcher::Exact(addr))
        }
    }

    fn matches(&self, ip: IpAddr) -> bool {
        match self {
            IpMatcher::Exact(addr) => ip == *addr,
            IpMatcher::Cidr(network, prefix) => cidr_contains(*network, *prefix, ip),
        }
    }
}

fn cidr_contains(network: IpAddr, prefix: u8, ip: IpAddr) -> bool {
    match (network, ip) {
        (IpAddr::V4(net), IpAddr::V4(addr)) => {
            if prefix >= 32 {
                return net == addr;
            }
            let mask = u32::MAX << (32 - prefix);
            (u32::from(net) & mask) == (u32::from(addr) & mask)
        }
        (IpAddr::V6(net), IpAddr::V6(addr)) => {
            if prefix >= 128 {
                return net == addr;
            }
            let mask = u128::MAX << (128 - prefix);
            (u128::from(net) & mask) == (u128::from(addr) & mask)
        }
        _ => false,
    }
}

struct PodMatcher {
    namespace: Option<String>,
    name_prefix: Option<String>,
}

impl PodMatcher {
    /// Parse "ns/prefix", "/prefix", "ns/", or "prefix" formats.
    fn parse(s: &str) -> Self {
        if let Some((ns, name)) = s.split_once('/') {
            Self {
                namespace: if ns.is_empty() { None } else { Some(ns.to_string()) },
                name_prefix: if name.is_empty() {
                    None
                } else {
                    Some(name.to_string())
                },
            }
        } else {
            // No slash — treat as name prefix in any namespace.
            Self {
                namespace: None,
                name_prefix: Some(s.to_string()),
            }
        }
    }

    fn matches(&self, namespace: &str, pod_name: &str) -> bool {
        if let Some(ref ns) = self.namespace {
            if namespace != ns {
                return false;
            }
        }
        if let Some(ref prefix) = self.name_prefix {
            if !pod_name.starts_with(prefix.as_str()) {
                return false;
            }
        }
        true
    }
}

struct NodeNameMatcher {
    pattern: String,
}

impl NodeNameMatcher {
    fn parse(s: &str) -> Self {
        Self {
            pattern: s.to_string(),
        }
    }

    /// Simple glob matching with `*` wildcard.
    fn matches(&self, name: &str) -> bool {
        glob_match(&self.pattern, name)
    }
}

/// Simple glob match supporting `*` as "match any sequence of characters".
fn glob_match(pattern: &str, text: &str) -> bool {
    let parts: Vec<&str> = pattern.split('*').collect();
    if parts.len() == 1 {
        // No wildcard — exact match.
        return pattern == text;
    }

    let mut pos = 0;
    for (i, part) in parts.iter().enumerate() {
        if part.is_empty() {
            continue;
        }
        if i == 0 {
            // First segment must match at the start.
            if !text.starts_with(part) {
                return false;
            }
            pos = part.len();
        } else if i == parts.len() - 1 {
            // Last segment must match at the end.
            if !text[pos..].ends_with(part) {
                return false;
            }
            pos = text.len();
        } else {
            // Middle segment — find anywhere after current pos.
            match text[pos..].find(part) {
                Some(idx) => pos += idx + part.len(),
                None => return false,
            }
        }
    }
    true
}

fn extract_source_port(flow: &Flow) -> Option<u32> {
    match flow.l4.as_ref()?.protocol.as_ref()? {
        flow::layer4::Protocol::Tcp(tcp) => Some(tcp.source_port),
        flow::layer4::Protocol::Udp(udp) => Some(udp.source_port),
        flow::layer4::Protocol::Sctp(sctp) => Some(sctp.source_port),
        _ => None,
    }
}

fn extract_destination_port(flow: &Flow) -> Option<u32> {
    match flow.l4.as_ref()?.protocol.as_ref()? {
        flow::layer4::Protocol::Tcp(tcp) => Some(tcp.destination_port),
        flow::layer4::Protocol::Udp(udp) => Some(udp.destination_port),
        flow::layer4::Protocol::Sctp(sctp) => Some(sctp.destination_port),
        _ => None,
    }
}

/// Check if all flags set in `filter` are also set in `flow_flags`.
fn tcp_flags_subset(filter: &TcpFlags, flow_flags: &TcpFlags) -> bool {
    (!filter.fin || flow_flags.fin)
        && (!filter.syn || flow_flags.syn)
        && (!filter.rst || flow_flags.rst)
        && (!filter.psh || flow_flags.psh)
        && (!filter.ack || flow_flags.ack)
        && (!filter.urg || flow_flags.urg)
        && (!filter.ece || flow_flags.ece)
        && (!filter.cwr || flow_flags.cwr)
        && (!filter.ns || flow_flags.ns)
}

#[cfg(test)]
mod tests {
    use super::*;
    use retina_proto::flow::{IpVersion, TrafficDirection, Verdict};

    fn make_flow() -> Flow {
        #[allow(deprecated)]
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
                identity: 100,
                ..Default::default()
            }),
            destination: Some(flow::Endpoint {
                namespace: "kube-system".to_string(),
                pod_name: "coredns-xyz".to_string(),
                labels: vec!["k8s-app=kube-dns".to_string()],
                identity: 200,
                ..Default::default()
            }),
            verdict: Verdict::Forwarded.into(),
            traffic_direction: TrafficDirection::Egress.into(),
            is_reply: Some(false),
            node_name: "node-1".to_string(),
            ..Default::default()
        }
    }

    #[test]
    fn empty_filter_passes_all() {
        let fs = FlowFilterSet::compile(&[], &[]);
        assert!(fs.is_empty());
        assert!(fs.matches(&make_flow()));
    }

    #[test]
    fn whitelist_source_ip_exact() {
        let wl = vec![FlowFilter {
            source_ip: vec!["10.0.0.1".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));

        let wl_miss = vec![FlowFilter {
            source_ip: vec!["10.0.0.99".to_string()],
            ..Default::default()
        }];
        let fs_miss = FlowFilterSet::compile(&wl_miss, &[]);
        assert!(!fs_miss.matches(&make_flow()));
    }

    #[test]
    fn whitelist_source_ip_cidr() {
        let wl = vec![FlowFilter {
            source_ip: vec!["10.0.0.0/24".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));

        let wl_miss = vec![FlowFilter {
            source_ip: vec!["192.168.0.0/16".to_string()],
            ..Default::default()
        }];
        let fs_miss = FlowFilterSet::compile(&wl_miss, &[]);
        assert!(!fs_miss.matches(&make_flow()));
    }

    #[test]
    fn blacklist_excludes() {
        let bl = vec![FlowFilter {
            source_ip: vec!["10.0.0.1".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&[], &bl);
        assert!(!fs.matches(&make_flow()));
    }

    #[test]
    fn whitelist_minus_blacklist() {
        let wl = vec![FlowFilter {
            source_ip: vec!["10.0.0.0/24".to_string()],
            ..Default::default()
        }];
        let bl = vec![FlowFilter {
            source_ip: vec!["10.0.0.1".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &bl);
        assert!(!fs.matches(&make_flow()));
    }

    #[test]
    fn filter_source_pod() {
        let wl = vec![FlowFilter {
            source_pod: vec!["default/web".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));
    }

    #[test]
    fn filter_destination_pod_namespace_only() {
        let wl = vec![FlowFilter {
            destination_pod: vec!["kube-system/".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));
    }

    #[test]
    fn filter_verdict() {
        let wl = vec![FlowFilter {
            verdict: vec![Verdict::Forwarded.into()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));

        let wl_miss = vec![FlowFilter {
            verdict: vec![Verdict::Dropped.into()],
            ..Default::default()
        }];
        let fs_miss = FlowFilterSet::compile(&wl_miss, &[]);
        assert!(!fs_miss.matches(&make_flow()));
    }

    #[test]
    fn filter_protocol() {
        let wl = vec![FlowFilter {
            protocol: vec!["TCP".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));

        let wl_miss = vec![FlowFilter {
            protocol: vec!["udp".to_string()],
            ..Default::default()
        }];
        let fs_miss = FlowFilterSet::compile(&wl_miss, &[]);
        assert!(!fs_miss.matches(&make_flow()));
    }

    #[test]
    fn filter_destination_port() {
        let wl = vec![FlowFilter {
            destination_port: vec!["80".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));
    }

    #[test]
    fn filter_tcp_flags() {
        let wl = vec![FlowFilter {
            tcp_flags: vec![TcpFlags {
                syn: true,
                ..Default::default()
            }],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));
    }

    #[test]
    fn filter_label_match() {
        let wl = vec![FlowFilter {
            source_label: vec!["app=web".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));
    }

    #[test]
    fn filter_label_no_match() {
        let wl = vec![FlowFilter {
            source_label: vec!["app=nonexistent".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(!fs.matches(&make_flow()));
    }

    #[test]
    fn filter_node_name_glob() {
        let wl = vec![FlowFilter {
            node_name: vec!["node-*".to_string()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));
    }

    #[test]
    fn filter_identity() {
        let wl = vec![FlowFilter {
            source_identity: vec![100],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));

        let wl_miss = vec![FlowFilter {
            source_identity: vec![999],
            ..Default::default()
        }];
        let fs_miss = FlowFilterSet::compile(&wl_miss, &[]);
        assert!(!fs_miss.matches(&make_flow()));
    }

    #[test]
    fn and_within_filter() {
        // source_ip AND destination_port must both match
        let wl = vec![FlowFilter {
            source_ip: vec!["10.0.0.1".to_string()],
            destination_port: vec!["443".to_string()], // flow has port 80
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(!fs.matches(&make_flow()));
    }

    #[test]
    fn or_across_whitelist_filters() {
        // Two filters: first doesn't match, second does
        let wl = vec![
            FlowFilter {
                source_ip: vec!["192.168.0.1".to_string()],
                ..Default::default()
            },
            FlowFilter {
                source_ip: vec!["10.0.0.1".to_string()],
                ..Default::default()
            },
        ];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));
    }

    #[test]
    fn glob_match_exact() {
        assert!(glob_match("node-1", "node-1"));
        assert!(!glob_match("node-1", "node-2"));
    }

    #[test]
    fn glob_match_trailing_star() {
        assert!(glob_match("node-*", "node-1"));
        assert!(glob_match("node-*", "node-abc"));
        assert!(!glob_match("node-*", "other-1"));
    }

    #[test]
    fn glob_match_leading_star() {
        assert!(glob_match("*.example.com", "foo.example.com"));
        assert!(!glob_match("*.example.com", "foo.other.com"));
    }

    #[test]
    fn glob_match_middle_star() {
        assert!(glob_match("cluster-*/node-*", "cluster-a/node-1"));
        assert!(!glob_match("cluster-*/node-*", "cluster-a/pod-1"));
    }

    #[test]
    fn filter_reply() {
        let wl = vec![FlowFilter {
            reply: vec![false],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));

        let wl_miss = vec![FlowFilter {
            reply: vec![true],
            ..Default::default()
        }];
        let fs_miss = FlowFilterSet::compile(&wl_miss, &[]);
        assert!(!fs_miss.matches(&make_flow()));
    }

    #[test]
    fn filter_ip_version() {
        let wl = vec![FlowFilter {
            ip_version: vec![IpVersion::IPv4.into()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));

        let wl_miss = vec![FlowFilter {
            ip_version: vec![IpVersion::IPv6.into()],
            ..Default::default()
        }];
        let fs_miss = FlowFilterSet::compile(&wl_miss, &[]);
        assert!(!fs_miss.matches(&make_flow()));
    }

    #[test]
    fn filter_traffic_direction() {
        let wl = vec![FlowFilter {
            traffic_direction: vec![TrafficDirection::Egress.into()],
            ..Default::default()
        }];
        let fs = FlowFilterSet::compile(&wl, &[]);
        assert!(fs.matches(&make_flow()));
    }
}
