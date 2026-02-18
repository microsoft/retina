use std::os::fd::AsRawFd;
use std::sync::Arc;

use aya::maps::{MapData, PerfEventArray};
use bytes::BytesMut;
use retina_common::PacketEvent;
use tokio::io::unix::AsyncFd;
use tokio::io::Interest;
use tokio::sync::broadcast;
use tracing::{debug, warn};

use retina_core::ipcache::IpCache;
use retina_core::metrics::{AgentState, ForwardLabels, LostEventLabels, Metrics, PerfReaderGuard};
use retina_core::store::FlowStore;

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
        let mut buf = perf_array.open(cpu_id, Some(256))?;
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

            let mut buffers = (0..16)
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

                                metrics.parsed_packets_counter.inc();

                                let mut hubble_flow = retina_core::flow::packet_event_to_flow(&pkt, boot_offset_ns);
                                retina_core::enricher::enrich_flow(&mut hubble_flow, &cache);

                                let labels = ForwardLabels::from_flow(&hubble_flow);
                                metrics.forward_count.get_or_create(&labels).inc();
                                metrics
                                    .forward_bytes
                                    .get_or_create(&labels)
                                    .inc_by(pkt.bytes as i64);
                                metrics.touch_forward(&labels);

                                let flow_arc = Arc::new(hubble_flow);

                                store.push(flow_arc.clone());
                                // Ignore send error (no subscribers).
                                let _ = tx.send(flow_arc);
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
