mod bench_helpers;

use std::sync::Arc;

use criterion::{black_box, criterion_group, criterion_main, BenchmarkId, Criterion};
use retina_core::store::FlowStore;
use retina_proto::flow::Flow;

fn bench_flow_store_push(c: &mut Criterion) {
    let mut group = c.benchmark_group("flow_store_push");

    for cap in [1_024, 4_096, 131_072] {
        let store = FlowStore::new(cap);
        // Pre-fill to capacity so we measure the eviction path.
        for _ in 0..cap {
            store.push(Arc::new(Flow::default()));
        }
        let flow = Arc::new(bench_helpers::make_enriched_flow());

        group.bench_with_input(BenchmarkId::from_parameter(cap), &cap, |b, _| {
            b.iter(|| store.push(black_box(flow.clone())))
        });
    }

    group.finish();
}

fn bench_flow_store_last_n(c: &mut Criterion) {
    let mut group = c.benchmark_group("flow_store_last_n");

    let store = FlowStore::new(131_072);
    for _ in 0..10_000 {
        store.push(Arc::new(bench_helpers::make_enriched_flow()));
    }

    for n in [100, 1_000] {
        group.bench_with_input(BenchmarkId::from_parameter(n), &n, |b, &n| {
            b.iter(|| store.last_n(black_box(n)))
        });
    }

    group.finish();
}

criterion_group!(benches, bench_flow_store_push, bench_flow_store_last_n);
criterion_main!(benches);
