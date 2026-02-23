use std::collections::BTreeMap;
use std::net::Ipv4Addr;

use prost::Message;
use prost_types::Timestamp;
use retina_common::*;
use retina_proto::flow;

// Cilium monitor API message types (from cilium/pkg/monitor/api/types.go).
const MESSAGE_TYPE_TRACE: i32 = 4;

// Cilium trace observation sub-types (from cilium/pkg/monitor/api/types.go).
const TRACE_TO_LXC: i32 = 0;
const TRACE_FROM_LXC: i32 = 5;
const TRACE_FROM_NETWORK: i32 = 10;
const TRACE_TO_NETWORK: i32 = 11;

/// Compute the offset (in nanoseconds) to convert CLOCK_BOOTTIME → CLOCK_REALTIME.
///
/// Call once at startup; the result stays valid for the process lifetime.
pub fn boot_to_realtime_offset() -> i64 {
    let mut boot = libc::timespec {
        tv_sec: 0,
        tv_nsec: 0,
    };
    let mut real = libc::timespec {
        tv_sec: 0,
        tv_nsec: 0,
    };
    unsafe {
        libc::clock_gettime(libc::CLOCK_BOOTTIME, &mut boot);
        libc::clock_gettime(libc::CLOCK_REALTIME, &mut real);
    }
    let boot_ns = boot.tv_sec * 1_000_000_000 + boot.tv_nsec;
    let real_ns = real.tv_sec * 1_000_000_000 + real.tv_nsec;
    real_ns - boot_ns
}

/// Convert a PacketEvent from eBPF into a Hubble Flow protobuf.
///
/// `boot_offset_ns` is the value returned by [`boot_to_realtime_offset`] and
/// converts the kernel CLOCK_BOOTTIME timestamp to wall-clock time.
pub fn packet_event_to_flow(pkt: &PacketEvent, boot_offset_ns: i64) -> flow::Flow {
    let wall_ns = pkt.ts_ns as i64 + boot_offset_ns;
    let secs = wall_ns / 1_000_000_000;
    let nanos = (wall_ns % 1_000_000_000) as i32;

    let src_ip = Ipv4Addr::from(pkt.src_ip).to_string();
    let dst_ip = Ipv4Addr::from(pkt.dst_ip).to_string();

    let ip = Some(flow::Ip {
        source: src_ip,
        destination: dst_ip,
        source_xlated: String::new(),
        ip_version: flow::IpVersion::IPv4.into(),
        encrypted: false,
    });

    let tcp_flags = tcp_flags_to_proto(pkt.flags);

    let (l4, summary) = match pkt.proto {
        IPPROTO_TCP => (
            Some(flow::Layer4 {
                protocol: Some(flow::layer4::Protocol::Tcp(flow::Tcp {
                    source_port: pkt.src_port as u32,
                    destination_port: pkt.dst_port as u32,
                    flags: Some(tcp_flags),
                })),
            }),
            tcp_flags_summary(&tcp_flags),
        ),
        IPPROTO_UDP => (
            Some(flow::Layer4 {
                protocol: Some(flow::layer4::Protocol::Udp(flow::Udp {
                    source_port: pkt.src_port as u32,
                    destination_port: pkt.dst_port as u32,
                })),
            }),
            "UDP".to_string(),
        ),
        _ => (None, String::new()),
    };

    let (trace_observation_point, trace_sub_type) = match pkt.observation_point {
        OBS_FROM_ENDPOINT => (flow::TraceObservationPoint::FromEndpoint, TRACE_FROM_LXC),
        OBS_TO_ENDPOINT => (flow::TraceObservationPoint::ToEndpoint, TRACE_TO_LXC),
        OBS_FROM_NETWORK => (flow::TraceObservationPoint::FromNetwork, TRACE_FROM_NETWORK),
        OBS_TO_NETWORK => (flow::TraceObservationPoint::ToNetwork, TRACE_TO_NETWORK),
        _ => (flow::TraceObservationPoint::UnknownPoint, 0),
    };

    let traffic_direction = match pkt.traffic_direction {
        DIR_INGRESS => flow::TrafficDirection::Ingress,
        DIR_EGRESS => flow::TrafficDirection::Egress,
        _ => flow::TrafficDirection::Unknown,
    };

    let is_reply = Some(pkt.is_reply != 0);

    let event_type = Some(flow::CiliumEventType {
        r#type: MESSAGE_TYPE_TRACE,
        sub_type: trace_sub_type,
    });

    let extensions = make_extensions(pkt.bytes);

    #[allow(deprecated)]
    flow::Flow {
        time: Some(Timestamp {
            seconds: secs,
            nanos,
        }),
        verdict: flow::Verdict::Forwarded.into(),
        ip,
        l4,
        r#type: flow::FlowType::L3L4.into(),
        node_name: String::new(),
        is_reply,
        event_type,
        trace_observation_point: trace_observation_point.into(),
        traffic_direction: traffic_direction.into(),
        extensions,
        summary,
        ..Default::default()
    }
}

/// Build the extensions Any field containing packet byte count.
#[doc(hidden)]
pub fn make_extensions(bytes: u32) -> Option<prost_types::Any> {
    if bytes == 0 {
        return None;
    }
    let s = prost_types::Struct {
        fields: BTreeMap::from([(
            "bytes".to_string(),
            prost_types::Value {
                kind: Some(prost_types::value::Kind::NumberValue(bytes as f64)),
            },
        )]),
    };
    let mut buf = Vec::with_capacity(s.encoded_len());
    s.encode(&mut buf).ok()?;
    Some(prost_types::Any {
        type_url: "type.googleapis.com/google.protobuf.Struct".to_string(),
        value: buf,
    })
}

/// Build a human-readable summary string from TCP flags (matching Go Retina).
#[doc(hidden)]
pub fn tcp_flags_summary(flags: &flow::TcpFlags) -> String {
    // Fast paths for the 3 most common patterns (>95% of real traffic).
    let other =
        flags.fin || flags.rst || flags.psh || flags.urg || flags.ece || flags.cwr || flags.ns;
    if !other {
        if flags.syn && flags.ack {
            return "TCP Flags: SYN-ACK".into();
        }
        if flags.syn {
            return "TCP Flags: SYN".into();
        }
        if flags.ack {
            return "TCP Flags: ACK".into();
        }
        return "TCP".into();
    }

    // General case: fixed-size array on the stack instead of Vec.
    let mut parts: [&str; 9] = [""; 9];
    let mut n = 0;
    macro_rules! push {
        ($cond:expr, $s:expr) => {
            if $cond {
                parts[n] = $s;
                n += 1;
            }
        };
    }
    if flags.syn && flags.ack {
        parts[n] = "SYN-ACK";
        n += 1;
    } else {
        push!(flags.syn, "SYN");
        push!(flags.ack, "ACK");
    }
    push!(flags.fin, "FIN");
    push!(flags.rst, "RST");
    push!(flags.psh, "PSH");
    push!(flags.urg, "URG");
    push!(flags.ece, "ECE");
    push!(flags.cwr, "CWR");
    push!(flags.ns, "NS");

    if n == 0 {
        "TCP".into()
    } else {
        // Pre-calculate capacity: "TCP Flags: " (12) + parts + separators.
        let cap = 12 + parts[..n].iter().map(|p| p.len()).sum::<usize>() + (n - 1) * 2;
        let mut s = String::with_capacity(cap);
        s.push_str("TCP Flags: ");
        for (i, part) in parts[..n].iter().enumerate() {
            if i > 0 {
                s.push_str(", ");
            }
            s.push_str(part);
        }
        s
    }
}

/// Convert a TCP flags bitmask into a Hubble TcpFlags proto.
#[doc(hidden)]
pub fn tcp_flags_to_proto(flags: u16) -> flow::TcpFlags {
    flow::TcpFlags {
        fin: (flags & TCP_FIN) != 0,
        syn: (flags & TCP_SYN) != 0,
        rst: (flags & TCP_RST) != 0,
        psh: (flags & TCP_PSH) != 0,
        ack: (flags & TCP_ACK) != 0,
        urg: (flags & TCP_URG) != 0,
        ece: (flags & TCP_ECE) != 0,
        cwr: (flags & TCP_CWR) != 0,
        ns: (flags & TCP_NS) != 0,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_pkt() -> PacketEvent {
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
            flags: TCP_SYN | TCP_ACK,
            is_reply: 0,
            ..Default::default()
        }
    }

    #[test]
    fn event_type_set() {
        let pkt = make_pkt();
        let f = packet_event_to_flow(&pkt, 0);
        let et = f.event_type.unwrap();
        assert_eq!(et.r#type, MESSAGE_TYPE_TRACE);
        assert_eq!(et.sub_type, TRACE_TO_LXC);
    }

    #[test]
    fn event_type_from_network() {
        let mut pkt = make_pkt();
        pkt.observation_point = OBS_FROM_NETWORK;
        let f = packet_event_to_flow(&pkt, 0);
        let et = f.event_type.unwrap();
        assert_eq!(et.sub_type, TRACE_FROM_NETWORK);
    }

    #[test]
    fn extensions_bytes() {
        let pkt = make_pkt();
        let f = packet_event_to_flow(&pkt, 0);
        let any = f.extensions.unwrap();
        assert_eq!(any.type_url, "type.googleapis.com/google.protobuf.Struct");
        let s = prost_types::Struct::decode(any.value.as_slice()).unwrap();
        let val = s.fields.get("bytes").unwrap();
        assert_eq!(
            val.kind,
            Some(prost_types::value::Kind::NumberValue(1500.0))
        );
    }

    #[test]
    fn extensions_none_when_zero_bytes() {
        let mut pkt = make_pkt();
        pkt.bytes = 0;
        let f = packet_event_to_flow(&pkt, 0);
        assert!(f.extensions.is_none());
    }

    #[allow(deprecated)]
    #[test]
    fn summary_tcp_flags() {
        let pkt = make_pkt(); // SYN | ACK
        let f = packet_event_to_flow(&pkt, 0);
        assert_eq!(f.summary, "TCP Flags: SYN-ACK");
    }

    #[allow(deprecated)]
    #[test]
    fn summary_tcp_syn_only() {
        let mut pkt = make_pkt();
        pkt.flags = TCP_SYN;
        let f = packet_event_to_flow(&pkt, 0);
        assert_eq!(f.summary, "TCP Flags: SYN");
    }

    #[allow(deprecated)]
    #[test]
    fn summary_tcp_no_flags() {
        let mut pkt = make_pkt();
        pkt.flags = 0;
        let f = packet_event_to_flow(&pkt, 0);
        assert_eq!(f.summary, "TCP");
    }

    #[allow(deprecated)]
    #[test]
    fn summary_udp() {
        let mut pkt = make_pkt();
        pkt.proto = IPPROTO_UDP;
        let f = packet_event_to_flow(&pkt, 0);
        assert_eq!(f.summary, "UDP");
    }

    #[allow(deprecated)]
    #[test]
    fn summary_tcp_fin_rst() {
        let mut pkt = make_pkt();
        pkt.flags = TCP_FIN | TCP_RST;
        let f = packet_event_to_flow(&pkt, 0);
        assert_eq!(f.summary, "TCP Flags: FIN, RST");
    }
}
