mod bench_helpers;

use criterion::{Criterion, black_box, criterion_group, criterion_main};
use retina_core::filter::FlowFilterSet;

fn bench_filter_compile(c: &mut Criterion) {
    let mut group = c.benchmark_group("filter_compile");

    let (ew, eb) = bench_helpers::empty_filters();
    group.bench_function("empty", |b| {
        b.iter(|| FlowFilterSet::compile(black_box(&ew), black_box(&eb)))
    });

    let (sw, sb) = bench_helpers::simple_whitelist();
    group.bench_function("simple", |b| {
        b.iter(|| FlowFilterSet::compile(black_box(&sw), black_box(&sb)))
    });

    let (cw, cb) = bench_helpers::complex_filters();
    group.bench_function("complex", |b| {
        b.iter(|| FlowFilterSet::compile(black_box(&cw), black_box(&cb)))
    });

    group.finish();
}

fn bench_filter_matches(c: &mut Criterion) {
    let mut group = c.benchmark_group("filter_matches");
    let flow = bench_helpers::make_enriched_flow();

    let (ew, eb) = bench_helpers::empty_filters();
    let empty_fs = FlowFilterSet::compile(&ew, &eb);
    group.bench_function("empty", |b| b.iter(|| empty_fs.matches(black_box(&flow))));

    let (sw, sb) = bench_helpers::simple_whitelist();
    let simple_fs = FlowFilterSet::compile(&sw, &sb);
    group.bench_function("simple_hit", |b| {
        b.iter(|| simple_fs.matches(black_box(&flow)))
    });

    let (cw, cb) = bench_helpers::complex_filters();
    let complex_fs = FlowFilterSet::compile(&cw, &cb);
    group.bench_function("complex", |b| {
        b.iter(|| complex_fs.matches(black_box(&flow)))
    });

    group.finish();
}

criterion_group!(benches, bench_filter_compile, bench_filter_matches);
criterion_main!(benches);
