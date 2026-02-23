use std::os::fd::AsRawFd;
use std::sync::Arc;

use aya::maps::{MapData, PerfEventArray, RingBuf};
use bytes::BytesMut;
use retina_common::PacketEvent;
use tokio::io::Interest;
use tokio::io::unix::AsyncFd;
use tokio::sync::broadcast;
use tracing::{debug, warn};

use retina_core::ipcache::IpCache;
use retina_core::metrics::{AgentState, ForwardLabels, LostEventLabels, Metrics, PerfReaderGuard};
use retina_core::store::FlowStore;

/// Number of pages per perf buffer (per CPU).
const PERF_BUFFER_PAGES: usize = 256;
/// Number of reusable read buffers per CPU reader.
const PERF_READ_BUFFERS: usize = 16;

/// Process a single PacketEvent: convert to Hubble Flow, enrich, broadcast, store.
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
    flow_store.push(flow_arc.clone());
    // Ignore send error (no subscribers).
    let _ = flow_tx.send(flow_arc);
}

/// Read perf events from all CPUs and convert to Hubble flows.
pub async fn run_perf_reader(
    mut perf_array: PerfEventArray<MapData>,
    flow_tx: broadcast::Sender<Arc<retina_proto::flow::Flow>>,
    flow_store: Arc<FlowStore>,
    ip_cache: Arc<IpCache>,
    metrics: Arc<Metrics>,
    state: Arc<AgentState>,
) -> anyhow::Result<()> {
    let boot_offset_ns = retina_core::flow::boot_to_realtime_offset();

    let cpus = aya::util::online_cpus().map_err(|e| anyhow::anyhow!("online_cpus: {:?}", e))?;
    let num_cpus = cpus.len();
    let mut handles = Vec::with_capacity(num_cpus);

    for cpu_id in cpus {
        let mut buf = perf_array.open(cpu_id, Some(PERF_BUFFER_PAGES))?;
        let tx = flow_tx.clone();
        let store = flow_store.clone();
        let cache = ip_cache.clone();
        let metrics = metrics.clone();
        let state = state.clone();

        let handle = tokio::spawn(async move {
            // Track this perf reader as alive (decrements on drop).
            let _guard = PerfReaderGuard::new(state);

            // Wrap the perf buffer fd in AsyncFd for tokio polling.
            let fd = buf.as_raw_fd();
            let async_fd = match AsyncFd::with_interest(fd, Interest::READABLE) {
                Ok(afd) => afd,
                Err(e) => {
                    warn!(cpu = cpu_id, "failed to create AsyncFd: {}", e);
                    return;
                }
            };

            let mut buffers = (0..PERF_READ_BUFFERS)
                .map(|_| BytesMut::with_capacity(core::mem::size_of::<PacketEvent>() * 2))
                .collect::<Vec<_>>();

            loop {
                // Wait for the perf buffer to become readable.
                let mut guard = match async_fd.readable().await {
                    Ok(guard) => guard,
                    Err(e) => {
                        warn!(cpu = cpu_id, "readable() error: {}", e);
                        continue;
                    }
                };

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

                                // Safety: PacketEvent is repr(C) and we verified the size.
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

                // Clear readiness so we wait for the next epoll wakeup.
                guard.clear_ready();
            }
        });

        handles.push(handle);
    }

    // Wait for all CPU readers (they run forever).
    for h in handles {
        let _ = h.await;
    }

    Ok(())
}

/// Read events from a shared BPF ring buffer and convert to Hubble flows.
///
/// Unlike the perf reader (one task per CPU), this uses a single shared ring
/// buffer and a single reader task. The kernel handles multi-producer
/// synchronization; we only need one consumer.
pub async fn run_ring_reader(
    ring_buf: RingBuf<MapData>,
    flow_tx: broadcast::Sender<Arc<retina_proto::flow::Flow>>,
    flow_store: Arc<FlowStore>,
    ip_cache: Arc<IpCache>,
    metrics: Arc<Metrics>,
    state: Arc<AgentState>,
) -> anyhow::Result<()> {
    let boot_offset_ns = retina_core::flow::boot_to_realtime_offset();

    // Track this reader as alive (decrements on drop).
    let _guard = PerfReaderGuard::new(state);

    // RingBuf implements AsFd — wrap it for async polling.
    let mut async_fd = AsyncFd::with_interest(ring_buf, Interest::READABLE)?;

    loop {
        // Wait for the ring buffer to become readable.
        let mut guard = match async_fd.readable_mut().await {
            Ok(guard) => guard,
            Err(e) => {
                warn!("ring buffer readable() error: {}", e);
                continue;
            }
        };

        // Drain all available events.
        let rb = guard.get_inner_mut();
        while let Some(item) = rb.next() {
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

        // Clear readiness so we wait for the next epoll wakeup.
        guard.clear_ready();
    }
}
