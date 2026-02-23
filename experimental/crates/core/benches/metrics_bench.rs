mod bench_helpers;

use std::time::Duration;

use criterion::{black_box, criterion_group, criterion_main, Criterion};
use retina_core::metrics::{ForwardLabels, Metrics};

fn bench_forward_labels(c: &mut Criterion) {
    let flow = bench_helpers::make_enriched_flow();
    c.bench_function("forward_labels_from_flow", |b| {
        b.iter(|| ForwardLabels::from_flow(black_box(&flow)))
    });
}

fn bench_touch_forward(c: &mut Criterion) {
    let mut group = c.benchmark_group("touch_forward");
    let flow = bench_helpers::make_enriched_flow();

    // Cold: always the same key (DashMap hit after first insert).
    group.bench_function("cold", |b| {
        let metrics = Metrics::new();
        b.iter(|| {
            let labels = ForwardLabels::from_flow(&flow);
            metrics.touch_forward(black_box(labels));
        })
    });

    group.finish();
}

fn bench_sweep_stale(c: &mut Criterion) {
    let mut group = c.benchmark_group("sweep_stale");

    // No stale entries (all recently touched).
    group.bench_function("0_stale", |b| {
        let metrics = Metrics::new();
        // Pre-populate with 100 distinct entries, all fresh.
        for i in 0..100u32 {
            let mut flow = bench_helpers::make_enriched_flow();
            flow.ip.as_mut().unwrap().source = format!("10.0.{}.{}", i / 256, i % 256);
            let labels = ForwardLabels::from_flow(&flow);
            metrics.touch_forward(labels);
        }
        b.iter(|| metrics.sweep_stale_forward(black_box(Duration::from_secs(300))))
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_forward_labels,
    bench_touch_forward,
    bench_sweep_stale
);
criterion_main!(benches);
