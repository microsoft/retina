mod bench_helpers;

use std::sync::Arc;

use criterion::{Criterion, black_box, criterion_group, criterion_main};
use retina_core::enricher::enrich_flow;
use retina_core::flow::packet_event_to_flow;
use retina_core::metrics::{ForwardLabels, Metrics};
use retina_core::store::FlowStore;
use tokio::sync::broadcast;

fn bench_full_pipeline(c: &mut Criterion) {
    let mut group = c.benchmark_group("pipeline");

    let cache = bench_helpers::make_populated_ipcache(1_000);
    let metrics = Arc::new(Metrics::new());
    let store = Arc::new(FlowStore::new(131_072));
    let (flow_tx, _rx) = broadcast::channel::<Arc<retina_proto::flow::Flow>>(4096);
    let pkt = bench_helpers::make_tcp_ack_event(); // Most common: data ACK

    // Full pipeline: convert → enrich → labels → metrics → store → broadcast.
    group.bench_function("packet_to_broadcast", |b| {
        b.iter(|| {
            let mut hubble_flow = packet_event_to_flow(black_box(&pkt), 0);
            enrich_flow(&mut hubble_flow, &cache);

            let labels = ForwardLabels::from_flow(&hubble_flow);
            metrics.forward_count.get_or_create(&labels).inc();
            metrics
                .forward_bytes
                .get_or_create(&labels)
                .inc_by(pkt.bytes as i64);
            metrics.touch_forward(labels);

            let flow_arc = Arc::new(hubble_flow);
            store.push(Arc::clone(&flow_arc));
            let _ = flow_tx.send(flow_arc);
        })
    });

    // Pipeline without enrichment (cache not synced).
    let unsynced_cache = retina_core::ipcache::IpCache::new();
    group.bench_function("packet_to_broadcast_no_enrich", |b| {
        b.iter(|| {
            let mut hubble_flow = packet_event_to_flow(black_box(&pkt), 0);
            enrich_flow(&mut hubble_flow, &unsynced_cache);
            let labels = ForwardLabels::from_flow(&hubble_flow);
            metrics.touch_forward(labels);
            let flow_arc = Arc::new(hubble_flow);
            store.push(Arc::clone(&flow_arc));
            let _ = flow_tx.send(flow_arc);
        })
    });

    // Pipeline + filter matching (simulates a connected Hubble client).
    let (wl, bl) = bench_helpers::complex_filters();
    let filter = retina_core::filter::FlowFilterSet::compile(&wl, &bl);
    group.bench_function("packet_to_filter", |b| {
        b.iter(|| {
            let mut hubble_flow = packet_event_to_flow(black_box(&pkt), 0);
            enrich_flow(&mut hubble_flow, &cache);
            let labels = ForwardLabels::from_flow(&hubble_flow);
            metrics.touch_forward(labels);
            let flow_arc = Arc::new(hubble_flow);
            store.push(Arc::clone(&flow_arc));
            let _ = filter.matches(&flow_arc);
            let _ = flow_tx.send(flow_arc);
        })
    });

    group.finish();
}

criterion_group!(benches, bench_full_pipeline);
criterion_main!(benches);
