use std::collections::BTreeMap;
use std::os::fd::{AsFd as _, AsRawFd};
use std::sync::Arc;

use aya::maps::{MapData, PerCpuHashMap, PerfEventArray, RingBuf};
use bytes::BytesMut;
use dropreason_common::{
    DropEvent, DropMetricsKey, DropMetricsValue, DropReason, DIR_EGRESS, DIR_INGRESS,
};
use prost::Message;
use prost_types::Timestamp;
use retina_core::ipcache::IpCache;
use retina_core::metrics::{AgentState, DropLabels, LostEventLabels, Metrics, PerfReaderGuard};
use retina_core::store::FlowStore;
use retina_proto::flow;
use tokio::sync::broadcast;
use tracing::{debug, warn};

/// Number of pages per perf buffer (per CPU). Drops are low-volume, so 64
/// pages is plenty (vs 256 for packetparser).
const PERF_BUFFER_PAGES: usize = 64;
/// Number of reusable read buffers per CPU reader.
const PERF_READ_BUFFERS: usize = 8;

// Cilium monitor API message type for drops.
const MESSAGE_TYPE_DROP: i32 = 1;

/// Block until the given fd is readable via `poll(2)`.
fn poll_readable(fd: i32) -> bool {
    let mut pfd = libc::pollfd {
        fd,
        events: libc::POLLIN,
        revents: 0,
    };
    loop {
        let ret = unsafe { libc::poll(&mut pfd, 1, -1) };
        if ret >= 0 {
            return true;
        }
        let err = std::io::Error::last_os_error();
        if err.kind() == std::io::ErrorKind::Interrupted {
            continue;
        }
        warn!("dropreason: poll error on fd {fd}: {err}");
        return false;
    }
}

fn direction_label(dir: u8) -> &'static str {
    match dir {
        DIR_INGRESS => "INGRESS",
        DIR_EGRESS => "EGRESS",
        _ => "TRAFFIC_DIRECTION_UNKNOWN",
    }
}

fn traffic_direction(dir: u8) -> flow::TrafficDirection {
    match dir {
        DIR_INGRESS => flow::TrafficDirection::Ingress,
        DIR_EGRESS => flow::TrafficDirection::Egress,
        _ => flow::TrafficDirection::Unknown,
    }
}

/// Map a kernel return code to a human-readable errno name.
fn errno_name(ret: i32) -> String {
    let name = match -ret {
        0 => "NF_DROP",
        1 => "EPERM",
        11 => "EAGAIN",
        13 => "EACCES",
        22 => "EINVAL",
        99 => "EADDRNOTAVAIL",
        101 => "ENETUNREACH",
        110 => "ETIMEDOUT",
        111 => "ECONNREFUSED",
        112 => "EHOSTDOWN",
        113 => "EHOSTUNREACH",
        125 => "ECANCELED",
        _ => return format!("ret={ret}"),
    };
    format!("{name} ({ret})")
}

/// Convert a [`DropEvent`] to a Hubble Flow with `verdict: DROPPED`.
fn drop_event_to_flow(event: &DropEvent, boot_offset_ns: i64) -> flow::Flow {
    let wall_ns = event.ts_ns as i64 + boot_offset_ns;
    let secs = wall_ns / 1_000_000_000;
    let nanos = (wall_ns % 1_000_000_000) as i32;

    let src_ip = std::net::Ipv4Addr::from(event.src_ip).to_string();
    let dst_ip = std::net::Ipv4Addr::from(event.dst_ip).to_string();

    let reason = DropReason::from_u8(event.drop_reason);

    let ip = Some(flow::Ip {
        source: src_ip,
        destination: dst_ip,
        source_xlated: String::new(),
        ip_version: flow::IpVersion::IPv4.into(),
        encrypted: false,
    });

    let l4 = match event.proto {
        6 => Some(flow::Layer4 {
            protocol: Some(flow::layer4::Protocol::Tcp(flow::Tcp {
                source_port: event.src_port as u32,
                destination_port: event.dst_port as u32,
                flags: None,
            })),
        }),
        17 => Some(flow::Layer4 {
            protocol: Some(flow::layer4::Protocol::Udp(flow::Udp {
                source_port: event.src_port as u32,
                destination_port: event.dst_port as u32,
            })),
        }),
        _ => None,
    };

    let ret = event.return_val as i32;
    let errno_str = errno_name(ret);
    let summary = format!("Drop: {} ({errno_str})", reason.as_str());

    // Build extensions with drop reason, return code, and byte count.
    let extensions = {
        let mut fields = BTreeMap::new();
        fields.insert(
            "drop_reason".to_string(),
            prost_types::Value {
                kind: Some(prost_types::value::Kind::StringValue(
                    reason.as_str().to_string(),
                )),
            },
        );
        fields.insert(
            "return_code".to_string(),
            prost_types::Value {
                kind: Some(prost_types::value::Kind::StringValue(errno_str)),
            },
        );
        if event.bytes > 0 {
            fields.insert(
                "bytes".to_string(),
                prost_types::Value {
                    kind: Some(prost_types::value::Kind::NumberValue(event.bytes as f64)),
                },
            );
        }
        let s = prost_types::Struct { fields };
        let mut buf = Vec::with_capacity(s.encoded_len());
        if s.encode(&mut buf).is_ok() {
            Some(prost_types::Any {
                type_url: "type.googleapis.com/google.protobuf.Struct".to_string(),
                value: buf,
            })
        } else {
            None
        }
    };

    #[allow(deprecated)]
    flow::Flow {
        time: Some(Timestamp {
            seconds: secs,
            nanos,
        }),
        verdict: flow::Verdict::Dropped.into(),
        drop_reason_desc: flow::DropReason::Unknown.into(),
        ip,
        l4,
        r#type: flow::FlowType::L3L4.into(),
        node_name: String::new(),
        is_reply: None,
        event_type: Some(flow::CiliumEventType {
            r#type: MESSAGE_TYPE_DROP,
            sub_type: 0,
        }),
        trace_observation_point: flow::TraceObservationPoint::FromNetwork.into(),
        traffic_direction: traffic_direction(event.direction).into(),
        extensions,
        summary,
        ..Default::default()
    }
}

/// Process a single DropEvent: convert to Hubble Flow, enrich, broadcast, store.
#[inline]
fn process_drop_event(
    event: &DropEvent,
    boot_offset_ns: i64,
    flow_tx: &broadcast::Sender<Arc<flow::Flow>>,
    flow_store: &FlowStore,
    ip_cache: &IpCache,
) {
    let mut hubble_flow = drop_event_to_flow(event, boot_offset_ns);
    retina_core::enricher::enrich_flow(&mut hubble_flow, ip_cache);

    let flow_arc = Arc::new(hubble_flow);
    flow_store.push(Arc::clone(&flow_arc));
    let _ = flow_tx.send(flow_arc);
}

/// Read events from a shared BPF ring buffer on a dedicated OS thread.
pub fn run_ring_reader(
    mut ring_buf: RingBuf<MapData>,
    flow_tx: broadcast::Sender<Arc<flow::Flow>>,
    flow_store: Arc<FlowStore>,
    ip_cache: Arc<IpCache>,
    state: Arc<AgentState>,
) {
    let boot_offset_ns = retina_core::flow::boot_to_realtime_offset();
    let _guard = PerfReaderGuard::new(state);
    let fd = ring_buf.as_fd().as_raw_fd();

    loop {
        if !poll_readable(fd) {
            continue;
        }

        while let Some(item) = ring_buf.next() {
            if item.len() < core::mem::size_of::<DropEvent>() {
                debug!(len = item.len(), "short dropreason ring event, skipping");
                continue;
            }

            let event: DropEvent =
                unsafe { core::ptr::read_unaligned(item.as_ptr() as *const DropEvent) };
            drop(item);

            process_drop_event(&event, boot_offset_ns, &flow_tx, &flow_store, &ip_cache);
        }
    }
}

/// Read events from per-CPU perf buffers on dedicated OS threads.
pub fn run_perf_reader(
    mut perf_array: PerfEventArray<MapData>,
    flow_tx: broadcast::Sender<Arc<flow::Flow>>,
    flow_store: Arc<FlowStore>,
    ip_cache: Arc<IpCache>,
    metrics: Arc<Metrics>,
    state: Arc<AgentState>,
) -> anyhow::Result<()> {
    let boot_offset_ns = retina_core::flow::boot_to_realtime_offset();

    let cpus = aya::util::online_cpus().map_err(|e| anyhow::anyhow!("online_cpus: {e:?}"))?;
    let mut handles = Vec::with_capacity(cpus.len());

    for cpu_id in cpus {
        let mut buf = perf_array.open(cpu_id, Some(PERF_BUFFER_PAGES))?;
        let tx = flow_tx.clone();
        let store = Arc::clone(&flow_store);
        let cache = Arc::clone(&ip_cache);
        let metrics = Arc::clone(&metrics);
        let state = Arc::clone(&state);

        let handle = std::thread::Builder::new()
            .name(format!("retina-drop-perf-{cpu_id}"))
            .spawn(move || {
                let _guard = PerfReaderGuard::new(state);
                let fd = buf.as_raw_fd();

                let mut buffers: Vec<BytesMut> = (0..PERF_READ_BUFFERS)
                    .map(|_| BytesMut::with_capacity(core::mem::size_of::<DropEvent>() * 2))
                    .collect();

                loop {
                    if !poll_readable(fd) {
                        continue;
                    }

                    loop {
                        match buf.read_events(&mut buffers) {
                            Ok(events) => {
                                if events.read == 0 {
                                    break;
                                }
                                if events.lost > 0 {
                                    metrics
                                        .lost_events_counter
                                        .get_or_create(&LostEventLabels {
                                            r#type: "perf".into(),
                                            reason: "dropreason_ring_full".into(),
                                        })
                                        .inc_by(events.lost as u64);
                                }
                                for b in buffers.iter().take(events.read) {
                                    if b.len() < core::mem::size_of::<DropEvent>() {
                                        continue;
                                    }
                                    let event: DropEvent = unsafe {
                                        core::ptr::read_unaligned(
                                            b.as_ptr() as *const DropEvent,
                                        )
                                    };
                                    process_drop_event(
                                        &event,
                                        boot_offset_ns,
                                        &tx,
                                        &store,
                                        &cache,
                                    );
                                }
                            }
                            Err(e) => {
                                warn!(cpu = cpu_id, "dropreason perf read error: {e}");
                                break;
                            }
                        }
                    }
                }
            })?;

        handles.push(handle);
    }

    for h in handles {
        let _ = h.join();
    }
    Ok(())
}

/// Periodically read the per-CPU metrics hash map and update Prometheus gauges.
///
/// This runs as a tokio task and updates `drop_count` / `drop_bytes` every 10
/// seconds. The per-CPU map is the authoritative source for aggregate metrics
/// (it never loses data, unlike ring buffer events which can be dropped).
pub async fn run_metrics_reader(
    metrics_map: PerCpuHashMap<MapData, DropMetricsKey, DropMetricsValue>,
    metrics: Arc<Metrics>,
) {
    let mut interval = tokio::time::interval(tokio::time::Duration::from_secs(10));

    loop {
        interval.tick().await;

        for item in metrics_map.iter() {
            match item {
                Ok((key, per_cpu_values)) => {
                    // Sum across all CPUs.
                    let mut total_count: u64 = 0;
                    let mut total_bytes: u64 = 0;
                    for val in per_cpu_values.iter() {
                        total_count += val.count;
                        total_bytes += val.bytes;
                    }

                    let reason = DropReason::from_u8(key.drop_reason);
                    let labels = DropLabels {
                        reason: reason.as_str().to_string(),
                        direction: direction_label(key.direction).to_string(),
                    };
                    metrics.drop_count.get_or_create(&labels).set(total_count as i64);
                    metrics.drop_bytes.get_or_create(&labels).set(total_bytes as i64);
                }
                Err(e) => {
                    debug!("dropreason metrics iter error: {e}");
                    break;
                }
            }
        }
    }
}
