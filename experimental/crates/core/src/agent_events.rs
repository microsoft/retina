use std::{
    collections::VecDeque,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc, RwLock,
    },
    time::Instant,
};

use retina_proto::flow::AgentEvent;

/// Ring buffer for storing recent agent events.
pub struct AgentEventStore {
    events: RwLock<VecDeque<Arc<AgentEvent>>>,
    capacity: usize,
    seen_events: AtomicU64,
    start_time: Instant,
}

impl AgentEventStore {
    pub fn new(capacity: usize) -> Self {
        Self {
            events: RwLock::new(VecDeque::with_capacity(capacity)),
            capacity,
            seen_events: AtomicU64::new(0),
            start_time: Instant::now(),
        }
    }

    pub fn push(&self, event: Arc<AgentEvent>) {
        self.seen_events.fetch_add(1, Ordering::Relaxed);
        let mut events = self.events.write().unwrap();
        if events.len() >= self.capacity {
            events.pop_front();
        }
        events.push_back(event);
    }

    pub fn last_n(&self, n: usize) -> Vec<Arc<AgentEvent>> {
        let events = self.events.read().unwrap();
        let start = events.len().saturating_sub(n);
        events.iter().skip(start).cloned().collect()
    }

    pub fn first_n(&self, n: usize) -> Vec<Arc<AgentEvent>> {
        let events = self.events.read().unwrap();
        events.iter().take(n).cloned().collect()
    }

    pub fn num_events(&self) -> u64 {
        self.events.read().unwrap().len() as u64
    }

    pub fn seen_events(&self) -> u64 {
        self.seen_events.load(Ordering::Relaxed)
    }

    pub fn uptime_ns(&self) -> u64 {
        self.start_time.elapsed().as_nanos() as u64
    }
}

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
        let store = AgentEventStore::new(10);
        store.push(Arc::new(make_event(AgentEventType::AgentStarted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheUpserted)));

        assert_eq!(store.num_events(), 2);
        assert_eq!(store.seen_events(), 2);

        let events = store.last_n(10);
        assert_eq!(events.len(), 2);
    }

    #[test]
    fn ring_buffer_eviction() {
        let store = AgentEventStore::new(2);
        store.push(Arc::new(make_event(AgentEventType::AgentStarted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheUpserted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheDeleted)));

        assert_eq!(store.num_events(), 2);
        assert_eq!(store.seen_events(), 3);

        let events = store.last_n(10);
        assert_eq!(events.len(), 2);
        assert_eq!(events[0].r#type, i32::from(AgentEventType::IpcacheUpserted));
    }

    #[test]
    fn first_n() {
        let store = AgentEventStore::new(10);
        store.push(Arc::new(make_event(AgentEventType::AgentStarted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheUpserted)));
        store.push(Arc::new(make_event(AgentEventType::IpcacheDeleted)));

        let events = store.first_n(2);
        assert_eq!(events.len(), 2);
        assert_eq!(events[0].r#type, i32::from(AgentEventType::AgentStarted));
        assert_eq!(events[1].r#type, i32::from(AgentEventType::IpcacheUpserted));
    }
}
