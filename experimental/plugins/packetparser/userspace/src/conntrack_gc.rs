use std::sync::Arc;

use aya::maps::{HashMap, MapData};
use retina_common::{CtEntry, CtV4Key};
use retina_core::metrics::Metrics;
use tracing::debug;

/// Get current monotonic boot time in seconds.
fn boot_time_secs() -> u32 {
    let mut ts = libc::timespec {
        tv_sec: 0,
        tv_nsec: 0,
    };
    unsafe {
        libc::clock_gettime(libc::CLOCK_BOOTTIME, &mut ts);
    }
    ts.tv_sec as u32
}

/// Run periodic conntrack garbage collection.
/// Iterates the conntrack map and removes expired entries.
/// Also sweeps stale forward metric label sets every 60s (4 × 15s ticks).
pub async fn run_gc(mut conntrack: HashMap<MapData, CtV4Key, CtEntry>, metrics: Arc<Metrics>) {
    let mut interval = tokio::time::interval(tokio::time::Duration::from_secs(15));
    let mut sweep_counter: u32 = 0;

    loop {
        interval.tick().await;

        let now_secs = boot_time_secs();
        let mut total = 0u64;
        let mut evicted = 0u64;
        let mut keys_to_delete = Vec::new();

        // Accumulators for conntrack metadata.
        let mut total_pkts_tx = 0i64;
        let mut total_pkts_rx = 0i64;
        let mut total_bytes_tx = 0i64;
        let mut total_bytes_rx = 0i64;

        // Iterate all entries.
        for item in conntrack.iter() {
            total += 1;
            match item {
                Ok((key, entry)) => {
                    total_pkts_tx += entry.pkts_since_report_tx as i64;
                    total_pkts_rx += entry.pkts_since_report_rx as i64;
                    total_bytes_tx += entry.bytes_since_report_tx as i64;
                    total_bytes_rx += entry.bytes_since_report_rx as i64;

                    if now_secs >= entry.eviction_time {
                        keys_to_delete.push(key);
                    }
                }
                Err(e) => {
                    debug!("conntrack iter error: {}", e);
                    break;
                }
            }
        }

        // Delete expired entries.
        for key in &keys_to_delete {
            // Re-check before deleting (entry may have been refreshed by eBPF).
            if let Ok(entry) = conntrack.get(key, 0) {
                if now_secs >= entry.eviction_time {
                    let _ = conntrack.remove(key);
                    evicted += 1;
                }
            }
        }

        // Update conntrack gauges.
        let live = (total - evicted) as i64;
        metrics.conntrack_total_connections.set(live);
        metrics.conntrack_packets_tx.set(total_pkts_tx);
        metrics.conntrack_packets_rx.set(total_pkts_rx);
        metrics.conntrack_bytes_tx.set(total_bytes_tx);
        metrics.conntrack_bytes_rx.set(total_bytes_rx);

        debug!(
            total_connections = total,
            evicted = evicted,
            "conntrack GC completed"
        );

        // Sweep stale forward metric label sets every ~60s (4 × 15s ticks).
        sweep_counter += 1;
        if sweep_counter % 4 == 0 {
            metrics.sweep_stale_forward(std::time::Duration::from_secs(300));
        }
    }
}
