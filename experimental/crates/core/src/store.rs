use std::{
    collections::VecDeque,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc, Mutex, RwLock,
    },
    time::Instant,
};

use retina_proto::flow::Flow;

/// Ring buffer for storing recent flows.
pub struct FlowStore {
    flows: RwLock<VecDeque<Arc<Flow>>>,
    capacity: usize,
    seen_flows: AtomicU64,
    start_time: Instant,
    /// Snapshot for computing flows_rate over a sliding window.
    rate_snapshot: Mutex<(Instant, u64)>,
}

impl FlowStore {
    pub fn new(capacity: usize) -> Self {
        let now = Instant::now();
        Self {
            flows: RwLock::new(VecDeque::with_capacity(capacity)),
            capacity,
            seen_flows: AtomicU64::new(0),
            start_time: now,
            rate_snapshot: Mutex::new((now, 0)),
        }
    }

    pub fn push(&self, flow: Arc<Flow>) {
        self.seen_flows.fetch_add(1, Ordering::Relaxed);
        let mut flows = self.flows.write().unwrap();
        if flows.len() >= self.capacity {
            flows.pop_front();
        }
        flows.push_back(flow);
    }

    /// Return the most recent N flows (newest last).
    pub fn last_n(&self, n: usize) -> Vec<Arc<Flow>> {
        let flows = self.flows.read().unwrap();
        let start = flows.len().saturating_sub(n);
        flows.iter().skip(start).cloned().collect()
    }

    /// Return the earliest N flows (oldest first).
    pub fn first_n(&self, n: usize) -> Vec<Arc<Flow>> {
        let flows = self.flows.read().unwrap();
        flows.iter().take(n).cloned().collect()
    }

    /// Return all flows currently in the ring buffer.
    pub fn all_flows(&self) -> Vec<Arc<Flow>> {
        let flows = self.flows.read().unwrap();
        flows.iter().cloned().collect()
    }

    pub fn num_flows(&self) -> u64 {
        self.flows.read().unwrap().len() as u64
    }

    pub fn seen_flows(&self) -> u64 {
        self.seen_flows.load(Ordering::Relaxed)
    }

    pub fn uptime_ns(&self) -> u64 {
        self.start_time.elapsed().as_nanos() as u64
    }

    pub fn capacity(&self) -> usize {
        self.capacity
    }

    /// Compute approximate flows/sec over a sliding window (~60s).
    ///
    /// Each call resets the snapshot if the window has elapsed, so the rate
    /// stays fresh without a background task.
    pub fn flows_rate(&self) -> f64 {
        let now = Instant::now();
        let current_seen = self.seen_flows();
        let mut snapshot = self.rate_snapshot.lock().unwrap();
        let elapsed = now.duration_since(snapshot.0);

        if elapsed.as_secs() < 5 {
            // Too little time has passed for a meaningful rate; return 0.
            return 0.0;
        }

        let delta = current_seen.saturating_sub(snapshot.1);
        let rate = delta as f64 / elapsed.as_secs_f64();

        // Reset snapshot every ~60s for a sliding window.
        if elapsed.as_secs() >= 60 {
            *snapshot = (now, current_seen);
        }

        rate
    }
}
