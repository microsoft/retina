use std::os::fd::{AsFd as _, AsRawFd};
use std::sync::Arc;

use aya::maps::{MapData, PerfEventArray, RingBuf};
use bytes::BytesMut;
use retina_common::PacketEvent;
use retina_core::ebpf::poll_readable;
use retina_core::ipcache::IpCache;
use retina_core::metrics::{AgentState, ForwardLabels, LostEventLabels, Metrics, PerfReaderGuard};
use retina_core::store::FlowStore;
use tokio::sync::broadcast;
use tracing::{debug, warn};

/// Number of pages per perf buffer (per CPU).
const PERF_BUFFER_PAGES: usize = 256;
/// Number of reusable read buffers per CPU reader.
const PERF_READ_BUFFERS: usize = 16;

/// Process a single `PacketEvent`: convert to Hubble Flow, enrich, broadcast, store.
#[inline]
fn process_packet_event(
    pkt: &PacketEvent,
    boot_offset_ns: i64,
    flow_tx: &broadcast::Sender<Arc<retina_proto::flow::Flow>>,
    flow_store: &FlowStore,
    ip_cache: &IpCache,
    metrics: &Metrics,
) {
    metrics.parsed_packets_counter.inc();

    let mut hubble_flow = retina_core::flow::packet_event_to_flow(pkt, boot_offset_ns);
    retina_core::enricher::enrich_flow(&mut hubble_flow, ip_cache);

    let labels = ForwardLabels::from_flow(&hubble_flow);
    metrics.forward_count.get_or_create(&labels).inc();
    metrics
        .forward_bytes
        .get_or_create(&labels)
        .inc_by(pkt.bytes as i64);
    metrics.touch_forward(labels);

    let flow_arc = Arc::new(hubble_flow);
    flow_store.push(Arc::clone(&flow_arc));
    // Ignore send error (no subscribers).
    let _ = flow_tx.send(flow_arc);
}

/// Read perf events from all CPUs on dedicated OS threads.
///
/// Spawns one thread per online CPU. Each thread blocks on `poll(2)` waiting
/// for its per-CPU perf buffer, then drains events synchronously.
pub(crate) fn run_perf_reader(
    mut perf_array: PerfEventArray<MapData>,
    flow_tx: broadcast::Sender<Arc<retina_proto::flow::Flow>>,
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
            .name(format!("retina-perf-{cpu_id}"))
            .spawn(move || {
                let _guard = PerfReaderGuard::new(state);
                let fd = buf.as_raw_fd();

                let mut buffers = (0..PERF_READ_BUFFERS)
                    .map(|_| BytesMut::with_capacity(core::mem::size_of::<PacketEvent>() * 2))
                    .collect::<Vec<_>>();

                loop {
                    if !poll_readable(fd) {
                        continue;
                    }

                    // Drain all available events.
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
                                            reason: "ring_full".into(),
                                        })
                                        .inc_by(events.lost as u64);
                                }
                                for b in buffers.iter().take(events.read) {
                                    if b.len() < core::mem::size_of::<PacketEvent>() {
                                        debug!(
                                            cpu = cpu_id,
                                            len = b.len(),
                                            "short perf event, skipping"
                                        );
                                        continue;
                                    }

                                    let pkt: PacketEvent = unsafe {
                                        core::ptr::read_unaligned(b.as_ptr() as *const PacketEvent)
                                    };

                                    process_packet_event(
                                        &pkt,
                                        boot_offset_ns,
                                        &tx,
                                        &store,
                                        &cache,
                                        &metrics,
                                    );
                                }
                            }
                            Err(e) => {
                                warn!(cpu = cpu_id, "perf read error: {}", e);
                                break;
                            }
                        }
                    }
                }
            })?;

        handles.push(handle);
    }

    // Block until all CPU reader threads exit (they run forever).
    for h in handles {
        let _ = h.join();
    }

    Ok(())
}

/// Read events from a shared BPF ring buffer on a dedicated OS thread.
///
/// Unlike the perf reader (one thread per CPU), this uses a single shared ring
/// buffer and a single reader thread. The kernel handles multi-producer
/// synchronization; we only need one consumer.
pub(crate) fn run_ring_reader(
    mut ring_buf: RingBuf<MapData>,
    flow_tx: broadcast::Sender<Arc<retina_proto::flow::Flow>>,
    flow_store: Arc<FlowStore>,
    ip_cache: Arc<IpCache>,
    metrics: Arc<Metrics>,
    state: Arc<AgentState>,
) {
    let boot_offset_ns = retina_core::flow::boot_to_realtime_offset();

    // Track this reader as alive (decrements on drop).
    let _guard = PerfReaderGuard::new(state);

    let fd = ring_buf.as_fd().as_raw_fd();

    loop {
        if !poll_readable(fd) {
            continue;
        }

        // Drain all available events.
        while let Some(item) = ring_buf.next() {
            if item.len() < core::mem::size_of::<PacketEvent>() {
                debug!(len = item.len(), "short ring buffer event, skipping");
                continue;
            }

            // Safety: PacketEvent is repr(C) and we verified the size.
            let pkt: PacketEvent =
                unsafe { core::ptr::read_unaligned(item.as_ptr() as *const PacketEvent) };

            // Drop item now to advance the consumer position promptly.
            drop(item);

            process_packet_event(
                &pkt,
                boot_offset_ns,
                &flow_tx,
                &flow_store,
                &ip_cache,
                &metrics,
            );
        }
    }
}
