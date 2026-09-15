mod bench_helpers;

use criterion::{Criterion, black_box, criterion_group, criterion_main};
use retina_core::enricher::{enrich_flow, identity_names};
use retina_core::flow::packet_event_to_flow;
use retina_core::ipcache::IpCache;

fn bench_enrich_flow(c: &mut Criterion) {
    let mut group = c.benchmark_group("enrich_flow");

    // Both IPs found in cache.
    let cache = bench_helpers::make_populated_ipcache(1000);
    let pkt = bench_helpers::make_tcp_syn_event(); // src=10.0.0.1, dst=10.0.0.2
    let base_flow = packet_event_to_flow(&pkt, 0);

    group.bench_function("both_found", |b| {
        b.iter(|| {
            let mut f = base_flow.clone();
            enrich_flow(black_box(&mut f), black_box(&cache));
            f
        })
    });

    // Both IPs miss (World identity).
    let empty_cache = IpCache::new();
    empty_cache.mark_synced();
    group.bench_function("miss_both", |b| {
        b.iter(|| {
            let mut f = base_flow.clone();
            enrich_flow(black_box(&mut f), black_box(&empty_cache));
            f
        })
    });

    // Cache not synced (early return).
    let unsynced_cache = IpCache::new();
    group.bench_function("not_synced", |b| {
        b.iter(|| {
            let mut f = base_flow.clone();
            enrich_flow(black_box(&mut f), black_box(&unsynced_cache));
            f
        })
    });

    group.finish();
}

fn bench_identity_names(c: &mut Criterion) {
    let mut group = c.benchmark_group("identity_names");

    let pod_id = bench_helpers::make_pod_identity("default", "nginx-abc123", &["app=nginx"]);
    group.bench_function("pod", |b| b.iter(|| identity_names(black_box(&pod_id))));

    let svc_id = bench_helpers::make_service_identity("default", "kubernetes");
    group.bench_function("service", |b| b.iter(|| identity_names(black_box(&svc_id))));

    let node_id = bench_helpers::make_node_identity("node-1");
    group.bench_function("node", |b| b.iter(|| identity_names(black_box(&node_id))));

    group.finish();
}

criterion_group!(benches, bench_enrich_flow, bench_identity_names);
criterion_main!(benches);
