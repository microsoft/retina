use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::time::{Duration, Instant};

use dashmap::DashMap;

use prometheus_client::encoding::{EncodeLabelSet, EncodeLabelValue, LabelValueEncoder};
use prometheus_client::metrics::counter::Counter;
use prometheus_client::metrics::family::Family;
use prometheus_client::metrics::gauge::Gauge;
use prometheus_client::registry::Registry;
use retina_proto::flow;

// ── Label types ───────────────────────────────────────────────────────────────

#[derive(Clone, Debug, Hash, PartialEq, Eq, EncodeLabelSet)]
pub struct ForwardLabels {
    pub direction: Direction,
    pub source_ip: String,
    pub source_namespace: String,
    pub source_podname: String,
    pub source_workload_kind: String,
    pub source_workload_name: String,
    pub destination_ip: String,
    pub destination_namespace: String,
    pub destination_podname: String,
    pub destination_workload_kind: String,
    pub destination_workload_name: String,
}

impl ForwardLabels {
    /// Build forward labels from an enriched Hubble flow.
    pub fn from_flow(f: &flow::Flow) -> Self {
        let direction = match flow::TrafficDirection::try_from(f.traffic_direction) {
            Ok(flow::TrafficDirection::Ingress) => Direction::Ingress,
            Ok(flow::TrafficDirection::Egress) => Direction::Egress,
            _ => Direction::Unknown,
        };

        let (source_ip, destination_ip) = match f.ip.as_ref() {
            Some(ip) => (ip.source.clone(), ip.destination.clone()),
            None => (String::new(), String::new()),
        };

        let (source_namespace, source_podname, source_workload_kind, source_workload_name) =
            endpoint_fields(f.source.as_ref());
        let (
            destination_namespace,
            destination_podname,
            destination_workload_kind,
            destination_workload_name,
        ) = endpoint_fields(f.destination.as_ref());

        Self {
            direction,
            source_ip,
            source_namespace,
            source_podname,
            source_workload_kind,
            source_workload_name,
            destination_ip,
            destination_namespace,
            destination_podname,
            destination_workload_kind,
            destination_workload_name,
        }
    }
}

/// Extract namespace, pod name, and first workload kind/name from an endpoint.
fn endpoint_fields(ep: Option<&flow::Endpoint>) -> (String, String, String, String) {
    match ep {
        Some(ep) => {
            let (wl_kind, wl_name) = ep
                .workloads
                .first()
                .map(|w| (w.kind.clone(), w.name.clone()))
                .unwrap_or_default();
            (ep.namespace.clone(), ep.pod_name.clone(), wl_kind, wl_name)
        }
        None => Default::default(),
    }
}

#[derive(Clone, Debug, Hash, PartialEq, Eq, EncodeLabelSet)]
pub struct DropLabels {
    pub reason: String,
    pub direction: String,
}

#[derive(Clone, Debug, Hash, PartialEq, Eq, EncodeLabelSet)]
pub struct LostEventLabels {
    pub r#type: String,
    pub reason: String,
}

#[derive(Clone, Debug, Hash, PartialEq, Eq)]
pub enum Direction {
    Ingress,
    Egress,
    Unknown,
}

impl EncodeLabelValue for Direction {
    fn encode(&self, encoder: &mut LabelValueEncoder) -> Result<(), std::fmt::Error> {
        let s = match self {
            Direction::Ingress => "INGRESS",
            Direction::Egress => "EGRESS",
            Direction::Unknown => "TRAFFIC_DIRECTION_UNKNOWN",
        };
        EncodeLabelValue::encode(&s, encoder)
    }
}

// ── Metrics ───────────────────────────────────────────────────────────────────

pub struct Metrics {
    // Data-plane (networkobservability_*)
    pub forward_count: Family<ForwardLabels, Gauge>,
    pub forward_bytes: Family<ForwardLabels, Gauge>,
    pub drop_count: Family<DropLabels, Gauge>,
    pub drop_bytes: Family<DropLabels, Gauge>,
    pub conntrack_total_connections: Gauge,
    pub conntrack_packets_tx: Gauge,
    pub conntrack_packets_rx: Gauge,
    pub conntrack_bytes_tx: Gauge,
    pub conntrack_bytes_rx: Gauge,

    // Control-plane (controlplane_networkobservability_*)
    pub parsed_packets_counter: Counter,
    pub lost_events_counter: Family<LostEventLabels, Counter>,

    pub registry: Registry,

    // TTL tracking for forward metric label sets (DashMap for lock-per-shard concurrency).
    forward_last_seen: DashMap<ForwardLabels, Instant>,
}

impl Default for Metrics {
    fn default() -> Self {
        Self::new()
    }
}

impl Metrics {
    pub fn new() -> Self {
        let mut registry = Registry::default();

        let forward_count = Family::<ForwardLabels, Gauge>::default();
        let forward_bytes = Family::<ForwardLabels, Gauge>::default();
        let drop_count = Family::<DropLabels, Gauge>::default();
        let drop_bytes = Family::<DropLabels, Gauge>::default();
        let conntrack_total_connections = Gauge::default();
        let conntrack_packets_tx = Gauge::default();
        let conntrack_packets_rx = Gauge::default();
        let conntrack_bytes_tx = Gauge::default();
        let conntrack_bytes_rx = Gauge::default();

        // Register data-plane metrics.
        {
            let dp = registry.sub_registry_with_prefix("networkobservability");
            dp.register(
                "forward_count",
                "Forwarded packets by direction",
                forward_count.clone(),
            );
            dp.register(
                "forward_bytes",
                "Forwarded bytes by direction",
                forward_bytes.clone(),
            );
            dp.register(
                "drop_count",
                "Dropped packets by reason and direction",
                drop_count.clone(),
            );
            dp.register(
                "drop_bytes",
                "Dropped bytes by reason and direction",
                drop_bytes.clone(),
            );
            dp.register(
                "conntrack_total_connections",
                "Current conntrack entries",
                conntrack_total_connections.clone(),
            );
            dp.register(
                "conntrack_packets_tx",
                "Conntrack TX packets",
                conntrack_packets_tx.clone(),
            );
            dp.register(
                "conntrack_packets_rx",
                "Conntrack RX packets",
                conntrack_packets_rx.clone(),
            );
            dp.register(
                "conntrack_bytes_tx",
                "Conntrack TX bytes",
                conntrack_bytes_tx.clone(),
            );
            dp.register(
                "conntrack_bytes_rx",
                "Conntrack RX bytes",
                conntrack_bytes_rx.clone(),
            );
        }

        let parsed_packets_counter = Counter::default();
        let lost_events_counter = Family::<LostEventLabels, Counter>::default();

        // Register control-plane metrics.
        {
            let cp = registry.sub_registry_with_prefix("controlplane_networkobservability");
            cp.register(
                "parsed_packets_counter",
                "Packets parsed by packetparser",
                parsed_packets_counter.clone(),
            );
            cp.register(
                "lost_events_counter",
                "Events lost from perf ring buffers",
                lost_events_counter.clone(),
            );
        }

        Self {
            forward_count,
            forward_bytes,
            drop_count,
            drop_bytes,
            conntrack_total_connections,
            conntrack_packets_tx,
            conntrack_packets_rx,
            conntrack_bytes_tx,
            conntrack_bytes_rx,
            parsed_packets_counter,
            lost_events_counter,
            registry,
            forward_last_seen: DashMap::new(),
        }
    }

    /// Record that `labels` was just observed (updates TTL timestamp).
    /// Takes ownership to avoid cloning on the hot path.
    pub fn touch_forward(&self, labels: ForwardLabels) {
        self.forward_last_seen.insert(labels, Instant::now());
    }

    /// Remove forward metric label sets not seen within `ttl`.
    pub fn sweep_stale_forward(&self, ttl: Duration) {
        let now = Instant::now();
        let mut stale = Vec::new();
        self.forward_last_seen.retain(|labels, ts| {
            if now.duration_since(*ts) > ttl {
                stale.push(labels.clone());
                false
            } else {
                true
            }
        });
        for labels in &stale {
            self.forward_count.remove(labels);
            self.forward_bytes.remove(labels);
        }
    }
}

// ── Agent State ───────────────────────────────────────────────────────────────

/// Shared agent health/readiness state, updated by various components.
pub struct AgentState {
    pub plugin_started: AtomicBool,
    pub grpc_bound: AtomicBool,
    pub perf_readers_alive: AtomicUsize,
}

impl Default for AgentState {
    fn default() -> Self {
        Self::new()
    }
}

impl AgentState {
    pub fn new() -> Self {
        Self {
            plugin_started: AtomicBool::new(false),
            grpc_bound: AtomicBool::new(false),
            perf_readers_alive: AtomicUsize::new(0),
        }
    }

    pub fn is_plugin_started(&self) -> bool {
        self.plugin_started.load(Ordering::Acquire)
    }

    pub fn is_grpc_bound(&self) -> bool {
        self.grpc_bound.load(Ordering::Acquire)
    }

    pub fn perf_readers_alive(&self) -> usize {
        self.perf_readers_alive.load(Ordering::Acquire)
    }
}

/// Drop guard that decrements perf_readers_alive when a perf reader task exits.
pub struct PerfReaderGuard {
    state: std::sync::Arc<AgentState>,
}

impl PerfReaderGuard {
    pub fn new(state: std::sync::Arc<AgentState>) -> Self {
        state.perf_readers_alive.fetch_add(1, Ordering::Release);
        Self { state }
    }
}

impl Drop for PerfReaderGuard {
    fn drop(&mut self) {
        self.state
            .perf_readers_alive
            .fetch_sub(1, Ordering::Release);
    }
}
