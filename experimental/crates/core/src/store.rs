use std::{
    collections::VecDeque,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc, RwLock,
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
}

impl FlowStore {
    pub fn new(capacity: usize) -> Self {
        Self {
            flows: RwLock::new(VecDeque::with_capacity(capacity)),
            capacity,
            seen_flows: AtomicU64::new(0),
            start_time: Instant::now(),
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

    pub fn last_n(&self, n: usize) -> Vec<Arc<Flow>> {
        let flows = self.flows.read().unwrap();
        let start = flows.len().saturating_sub(n);
        flows.iter().skip(start).cloned().collect()
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
}
