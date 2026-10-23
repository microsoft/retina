use std::collections::{BTreeMap, HashMap, HashSet};
use std::net::Ipv4Addr;
use std::os::fd::{AsFd as _, AsRawFd};
use std::sync::Arc;

use aya::maps::{MapData, PerCpuArray, PerCpuHashMap, PerfEventArray, RingBuf};
use bytes::BytesMut;
use dropreason_common::{
    DIR_EGRESS, DIR_INGRESS, DropEvent, DropMetricsKey, DropMetricsValue, DropReason,
};
use prost::Message;
use prost_types::Timestamp;
use retina_core::ebpf::poll_readable;
use retina_core::ipcache::IpCache;
use retina_core::metrics::{
    AgentState, DropFlowLabels, DropLabels, LostEventLabels, Metrics, PerfReaderGuard,
};
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

/// Resolve the source pod IP from a process ID by reading its network namespace.
///
/// Parses `/proc/{pid}/net/fib_trie` looking for `/32 host LOCAL` entries to find
/// IPs assigned in the process's network namespace. Returns the first non-loopback
/// local IP (typically the pod's primary IP).
///
/// Returns `None` if the process has exited, /proc is unreadable, or no suitable IP
/// is found. This is best-effort — only called for drops with `src_ip == 0`.
fn resolve_src_ip_from_pid(pid: u32) -> Option<Ipv4Addr> {
    let path = format!("/proc/{pid}/net/fib_trie");
    let content = std::fs::read_to_string(path).ok()?;

    // Sliding window: when we see an IP on a `|-- x.x.x.x` line, check if the
    // NEXT line contains `/32 host LOCAL`. If so, it's a locally-assigned address.
    let mut last_ip: Option<Ipv4Addr> = None;
    for line in content.lines() {
        let trimmed = line.trim();
        if let Some(ip_str) = trimmed.strip_prefix("|-- ") {
            last_ip = ip_str.trim().parse::<Ipv4Addr>().ok();
        } else if trimmed.contains("/32 host LOCAL") {
            if let Some(ip) = last_ip {
                // Skip loopback (127.0.0.0/8).
                if ip.octets()[0] != 127 {
                    return Some(ip);
                }
            }
        } else {
            // Reset on non-matching lines (e.g. subnet headers).
            last_ip = None;
        }
    }
    None
}

/// Convert a [`DropEvent`] to a Hubble Flow with `verdict: DROPPED`.
fn drop_event_to_flow(
    event: &DropEvent,
    boot_offset_ns: i64,
    kernel_drop_reasons: &HashMap<u32, String>,
) -> flow::Flow {
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

    // Build summary string.
    let summary = match reason {
        // kfree_skb: show the kernel's own drop reason name.
        DropReason::KernelDrop => {
            let kernel_reason = kernel_drop_reasons
                .get(&event.kernel_drop_reason)
                .map_or("UNKNOWN", std::string::String::as_str);
            format!("Drop: {kernel_reason}")
        }
        // fexit hooks: show the kernel return code / errno.
        DropReason::IptableRuleDrop
        | DropReason::IptableNatDrop
        | DropReason::TcpConnectDrop
        | DropReason::TcpAcceptDrop
        | DropReason::ConntrackDrop => {
            let ret = event.return_val as i32;
            let errno_str = errno_name(ret);
            format!("Drop: {} ({errno_str})", reason.as_str())
        }
        // TCP tracepoints + unknown: just show the reason name.
        _ => format!("Drop: {}", reason.as_str()),
    };

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
        // For KernelDrop, include the specific kernel reason name.
        if reason == DropReason::KernelDrop && event.kernel_drop_reason > 0 {
            let kernel_reason = kernel_drop_reasons
                .get(&event.kernel_drop_reason)
                .cloned()
                .unwrap_or_else(|| format!("UNKNOWN_{}", event.kernel_drop_reason));
            fields.insert(
                "kernel_drop_reason".to_string(),
                prost_types::Value {
                    kind: Some(prost_types::value::Kind::StringValue(kernel_reason)),
                },
            );
        } else if matches!(
            reason,
            DropReason::IptableRuleDrop
                | DropReason::IptableNatDrop
                | DropReason::TcpConnectDrop
                | DropReason::TcpAcceptDrop
                | DropReason::ConntrackDrop
        ) {
            let ret = event.return_val as i32;
            let errno_str = errno_name(ret);
            fields.insert(
                "return_code".to_string(),
                prost_types::Value {
                    kind: Some(prost_types::value::Kind::StringValue(errno_str)),
                },
            );
        }
        if event.bytes > 0 {
            fields.insert(
                "bytes".to_string(),
                prost_types::Value {
                    kind: Some(prost_types::value::Kind::NumberValue(event.bytes as f64)),
                },
            );
        }
        if event.pid > 0 {
            fields.insert(
                "pid".to_string(),
                prost_types::Value {
                    kind: Some(prost_types::value::Kind::NumberValue(event.pid as f64)),
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
        is_reply: Some(false),
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

/// Process a single `DropEvent`: convert to Hubble Flow, enrich, broadcast, store,
/// and update per-flow drop metrics.
///
/// When `src_ip` is 0 (e.g. early `tcp_v4_connect` failure before source IP
/// assignment), attempts to resolve the source pod IP from the PID's network
/// namespace via `/proc/{pid}/net/fib_trie`.
#[inline]
#[allow(clippy::too_many_arguments)]
fn process_drop_event(
    event: &DropEvent,
    boot_offset_ns: i64,
    flow_tx: &broadcast::Sender<Arc<flow::Flow>>,
    flow_store: &FlowStore,
    ip_cache: &IpCache,
    metrics: &Metrics,
    kernel_drop_reasons: &HashMap<u32, String>,
    suppressed_reasons: &HashSet<String>,
) {
    // Check userspace suppress filter before doing any work.
    let reason = DropReason::from_u8(event.drop_reason);
    let reason_name = if reason == DropReason::KernelDrop {
        kernel_drop_reasons
            .get(&event.kernel_drop_reason)
            .map_or("", std::string::String::as_str)
    } else {
        reason.as_str()
    };
    if suppressed_reasons.contains(reason_name) {
        return;
    }

    // Resolve source IP from PID when the socket didn't have one.
    let mut patched = *event;
    if patched.src_ip == 0
        && patched.pid > 0
        && let Some(ip) = resolve_src_ip_from_pid(patched.pid)
    {
        patched.src_ip = u32::from(ip);
    }

    let mut hubble_flow = drop_event_to_flow(&patched, boot_offset_ns, kernel_drop_reasons);
    retina_core::enricher::enrich_flow(&mut hubble_flow, ip_cache);

    // Update per-flow drop metrics with full K8s context.
    let labels = DropFlowLabels::from_flow(reason_name, &hubble_flow);
    metrics.drop_flow_count.get_or_create(&labels).inc();
    metrics
        .drop_flow_bytes
        .get_or_create(&labels)
        .inc_by(patched.bytes as i64);
    metrics.touch_drop(labels);

    let flow_arc = Arc::new(hubble_flow);
    flow_store.push(Arc::clone(&flow_arc));
    let _ = flow_tx.send(flow_arc);
}

/// Read events from a shared BPF ring buffer on a dedicated OS thread.
#[allow(clippy::too_many_arguments)]
pub(crate) fn run_ring_reader(
    mut ring_buf: RingBuf<MapData>,
    flow_tx: broadcast::Sender<Arc<flow::Flow>>,
    flow_store: Arc<FlowStore>,
    ip_cache: Arc<IpCache>,
    metrics: Arc<Metrics>,
    state: Arc<AgentState>,
    kernel_drop_reasons: Arc<HashMap<u32, String>>,
    suppressed_reasons: Arc<HashSet<String>>,
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

            process_drop_event(
                &event,
                boot_offset_ns,
                &flow_tx,
                &flow_store,
                &ip_cache,
                &metrics,
                &kernel_drop_reasons,
                &suppressed_reasons,
            );
        }
    }
}

/// Read events from per-CPU perf buffers on dedicated OS threads.
#[allow(clippy::too_many_arguments)]
pub(crate) fn run_perf_reader(
    mut perf_array: PerfEventArray<MapData>,
    flow_tx: broadcast::Sender<Arc<flow::Flow>>,
    flow_store: Arc<FlowStore>,
    ip_cache: Arc<IpCache>,
    metrics: Arc<Metrics>,
    state: Arc<AgentState>,
    kernel_drop_reasons: Arc<HashMap<u32, String>>,
    suppressed_reasons: Arc<HashSet<String>>,
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
        let kdr = Arc::clone(&kernel_drop_reasons);
        let sr = Arc::clone(&suppressed_reasons);

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
                                        core::ptr::read_unaligned(b.as_ptr() as *const DropEvent)
                                    };
                                    process_drop_event(
                                        &event,
                                        boot_offset_ns,
                                        &tx,
                                        &store,
                                        &cache,
                                        &metrics,
                                        &kdr,
                                        &sr,
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
///
/// For fexit hooks, multiple eBPF map entries with different `return_val` may
/// share the same Prometheus label (e.g. `IPTABLE_RULE_DROP`). We accumulate
/// counts across all return values before setting the gauge, so no entry
/// overwrites another.
pub(crate) async fn run_metrics_reader(
    metrics_map: PerCpuHashMap<MapData, DropMetricsKey, DropMetricsValue>,
    metrics: Arc<Metrics>,
    kernel_drop_reasons: Arc<HashMap<u32, String>>,
    ring_lost_map: Option<PerCpuArray<MapData, u64>>,
) {
    let mut interval = tokio::time::interval(tokio::time::Duration::from_secs(10));
    let mut prev_ring_lost: u64 = 0;

    loop {
        interval.tick().await;

        // Accumulate across return_val variants that share the same label.
        let mut acc: HashMap<(String, String), (u64, u64)> = HashMap::new();

        for item in &metrics_map {
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

                    // For KernelDrop, resolve the specific kernel reason from
                    // return_val (which stores the skb_drop_reason enum value).
                    let reason_label = if reason == DropReason::KernelDrop {
                        kernel_drop_reasons
                            .get(&(key.return_val as u32))
                            .cloned()
                            .unwrap_or_else(|| format!("KERNEL_DROP_{}", key.return_val))
                    } else {
                        reason.as_str().to_string()
                    };

                    let direction = direction_label(key.direction).to_string();
                    let entry = acc.entry((reason_label, direction)).or_insert((0, 0));
                    entry.0 += total_count;
                    entry.1 += total_bytes;
                }
                Err(e) => {
                    debug!("dropreason metrics iter error: {e}");
                    break;
                }
            }
        }

        for ((reason_label, direction), (count, bytes)) in &acc {
            let labels = DropLabels {
                reason: reason_label.clone(),
                direction: direction.clone(),
            };
            metrics.drop_count.get_or_create(&labels).set(*count as i64);
            metrics.drop_bytes.get_or_create(&labels).set(*bytes as i64);
        }

        // Sweep stale per-flow drop metric label sets (5 min TTL).
        metrics.sweep_stale_drop(std::time::Duration::from_secs(300));

        // Report ring buffer lost events (delta since last tick).
        if let Some(ref map) = ring_lost_map
            && let Ok(per_cpu) = map.get(&0, 0)
        {
            let total: u64 = per_cpu.iter().sum();
            if total > prev_ring_lost {
                metrics
                    .lost_events_counter
                    .get_or_create(&LostEventLabels {
                        r#type: "ring".into(),
                        reason: "dropreason_ring_full".into(),
                    })
                    .inc_by(total - prev_ring_lost);
                prev_ring_lost = total;
            }
        }
    }
}
