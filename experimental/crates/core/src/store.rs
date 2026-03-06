//! Fixed-capacity ring buffer stores for flows and agent events.

use std::{
    collections::VecDeque,
    sync::{
        Arc, Mutex, RwLock,
        atomic::{AtomicU64, Ordering},
    },
    time::Instant,
};

use retina_proto::flow::{AgentEvent, Flow};

/// Generic ring buffer for storing recent items with a fixed capacity.
///
/// Thread-safe (RwLock-guarded) FIFO buffer that evicts the oldest item
/// when full. Tracks cumulative seen count and creation time.
pub struct RingBuffer<T> {
    items: RwLock<VecDeque<Arc<T>>>,
    capacity: usize,
    seen_items: AtomicU64,
    start_time: Instant,
}

impl<T> RingBuffer<T> {
    #[must_use]
    pub fn new(capacity: usize) -> Self {
        Self {
            items: RwLock::new(VecDeque::with_capacity(capacity)),
            capacity,
            seen_items: AtomicU64::new(0),
            start_time: Instant::now(),
        }
    }

    pub fn push(&self, item: Arc<T>) {
        self.seen_items.fetch_add(1, Ordering::Relaxed);
        let mut items = self.items.write().expect("lock poisoned");
        if items.len() >= self.capacity {
            items.pop_front();
        }
        items.push_back(item);
    }

    /// Return the most recent N items (newest last).
    pub fn last_n(&self, n: usize) -> Vec<Arc<T>> {
        let items = self.items.read().expect("lock poisoned");
        let start = items.len().saturating_sub(n);
        items.iter().skip(start).cloned().collect()
    }

    /// Return the earliest N items (oldest first).
    pub fn first_n(&self, n: usize) -> Vec<Arc<T>> {
        let items = self.items.read().expect("lock poisoned");
        items.iter().take(n).cloned().collect()
    }

    /// Return all items currently in the ring buffer.
    pub fn all(&self) -> Vec<Arc<T>> {
        let items = self.items.read().expect("lock poisoned");
        items.iter().cloned().collect()
    }

    /// Current number of items in the buffer.
    #[allow(clippy::len_without_is_empty)]
    pub fn len(&self) -> u64 {
        self.items.read().expect("lock poisoned").len() as u64
    }

    /// Cumulative count of items ever pushed.
    pub fn seen(&self) -> u64 {
        self.seen_items.load(Ordering::Relaxed)
    }

    /// Nanoseconds since this buffer was created.
    pub fn uptime_ns(&self) -> u64 {
        self.start_time.elapsed().as_nanos() as u64
    }

    pub fn capacity(&self) -> usize {
        self.capacity
    }
}

/// Ring buffer for recent flows, with flow-rate calculation.
pub struct FlowStore {
    inner: RingBuffer<Flow>,
    /// Snapshot for computing `flows_rate` over a sliding window.
    rate_snapshot: Mutex<(Instant, u64)>,
}

impl FlowStore {
    #[must_use]
    pub fn new(capacity: usize) -> Self {
        Self {
            inner: RingBuffer::new(capacity),
            rate_snapshot: Mutex::new((Instant::now(), 0)),
        }
    }

    pub fn push(&self, flow: Arc<Flow>) {
        self.inner.push(flow);
    }

    pub fn last_n(&self, n: usize) -> Vec<Arc<Flow>> {
        self.inner.last_n(n)
    }

    pub fn first_n(&self, n: usize) -> Vec<Arc<Flow>> {
        self.inner.first_n(n)
    }

    pub fn all_flows(&self) -> Vec<Arc<Flow>> {
        self.inner.all()
    }

    pub fn num_flows(&self) -> u64 {
        self.inner.len()
    }

    pub fn seen_flows(&self) -> u64 {
        self.inner.seen()
    }

    pub fn uptime_ns(&self) -> u64 {
        self.inner.uptime_ns()
    }

    pub fn capacity(&self) -> usize {
        self.inner.capacity()
    }

    /// Compute approximate flows/sec over a sliding window (~60s).
    ///
    /// Each call resets the snapshot if the window has elapsed, so the rate
    /// stays fresh without a background task.
    pub fn flows_rate(&self) -> f64 {
        let now = Instant::now();
        let current_seen = self.seen_flows();
        let mut snapshot = self.rate_snapshot.lock().expect("lock poisoned");
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

/// Ring buffer for recent agent events.
pub type AgentEventStore = RingBuffer<AgentEvent>;

#[cfg(test)]
mod tests {
    use super::*;
    use retina_proto::flow::AgentEventType;

    fn make_event(event_type: AgentEventType) -> AgentEvent {
        AgentEvent {
            r#type: event_type.into(),
            notification: None,
        }
    }

    #[test]
    fn push_and_retrieve() {
        let store = RingBuffer::<AgentEvent>::new(10);
        store.push(Arc::new(make_event(AgentEventType::AgentStarted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheUpserted)));

        assert_eq!(store.len(), 2);
        assert_eq!(store.seen(), 2);

        let events = store.last_n(10);
        assert_eq!(events.len(), 2);
    }

    #[test]
    fn ring_buffer_eviction() {
        let store = RingBuffer::<AgentEvent>::new(2);
        store.push(Arc::new(make_event(AgentEventType::AgentStarted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheUpserted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheDeleted)));

        assert_eq!(store.len(), 2);
        assert_eq!(store.seen(), 3);

        let events = store.last_n(10);
        assert_eq!(events.len(), 2);
        assert_eq!(events[0].r#type, i32::from(AgentEventType::IpcacheUpserted));
    }

    #[test]
    fn first_n() {
        let store = RingBuffer::<AgentEvent>::new(10);
        store.push(Arc::new(make_event(AgentEventType::AgentStarted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheUpserted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheDeleted)));

        let events = store.first_n(2);
        assert_eq!(events.len(), 2);
        assert_eq!(events[0].r#type, i32::from(AgentEventType::AgentStarted));
        assert_eq!(events[1].r#type, i32::from(AgentEventType::IpcacheUpserted));
    }

    #[test]
    fn all_returns_everything() {
        let store = RingBuffer::<AgentEvent>::new(10);
        store.push(Arc::new(make_event(AgentEventType::AgentStarted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheUpserted)));

        let all = store.all();
        assert_eq!(all.len(), 2);
    }

    #[test]
    fn capacity_is_correct() {
        let store = RingBuffer::<AgentEvent>::new(42);
        assert_eq!(store.capacity(), 42);
    }

    #[test]
    fn flow_store_push_and_retrieve() {
        let store = FlowStore::new(10);
        let flow = Arc::new(Flow::default());
        store.push(flow);

        assert_eq!(store.num_flows(), 1);
        assert_eq!(store.seen_flows(), 1);
        assert_eq!(store.all_flows().len(), 1);
    }

    #[test]
    fn flow_store_eviction() {
        let store = FlowStore::new(2);
        store.push(Arc::new(Flow::default()));
        store.push(Arc::new(Flow::default()));
        store.push(Arc::new(Flow::default()));

        assert_eq!(store.num_flows(), 2);
        assert_eq!(store.seen_flows(), 3);
    }

    #[test]
    fn flow_store_rate_returns_zero_initially() {
        let store = FlowStore::new(10);
        assert_eq!(store.flows_rate(), 0.0);
    }
}
