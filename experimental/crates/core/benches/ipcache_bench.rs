mod bench_helpers;

use std::net::{IpAddr, Ipv4Addr};
use std::sync::Arc;

use criterion::{BenchmarkId, Criterion, black_box, criterion_group, criterion_main};
use retina_core::ipcache::{Identity, Workload};

fn bench_ipcache_get(c: &mut Criterion) {
    let mut group = c.benchmark_group("ipcache_get");

    for size in [100, 1_000, 10_000] {
        let cache = bench_helpers::make_populated_ipcache(size);
        let hit_ip = IpAddr::V4(Ipv4Addr::new(10, 0, 0, 1));
        let miss_ip = IpAddr::V4(Ipv4Addr::new(192, 168, 99, 99));

        group.bench_with_input(BenchmarkId::new("hit", size), &size, |b, _| {
            b.iter(|| cache.get(black_box(&hit_ip)))
        });

        group.bench_with_input(BenchmarkId::new("miss", size), &size, |b, _| {
            b.iter(|| cache.get(black_box(&miss_ip)))
        });
    }

    group.finish();
}

fn bench_ipcache_get_pair(c: &mut Criterion) {
    let mut group = c.benchmark_group("ipcache_get_pair");

    for size in [100, 1_000, 10_000] {
        let cache = bench_helpers::make_populated_ipcache(size);
        let ip1 = IpAddr::V4(Ipv4Addr::new(10, 0, 0, 1));
        let ip2 = IpAddr::V4(Ipv4Addr::new(10, 0, 0, 2));
        let miss_ip = IpAddr::V4(Ipv4Addr::new(192, 168, 99, 99));

        group.bench_with_input(BenchmarkId::new("both_hit", size), &size, |b, _| {
            b.iter(|| cache.get_pair(black_box(&ip1), black_box(&ip2)))
        });

        group.bench_with_input(BenchmarkId::new("one_miss", size), &size, |b, _| {
            b.iter(|| cache.get_pair(black_box(&ip1), black_box(&miss_ip)))
        });
    }

    group.finish();
}

fn bench_numeric_identity(c: &mut Criterion) {
    let mut group = c.benchmark_group("identity_numeric");

    let pod_3 = bench_helpers::make_pod_identity(
        "default",
        "nginx-abc",
        &["app=nginx", "tier=frontend", "env=prod"],
    );
    group.bench_function("pod_3_labels", |b| {
        b.iter(|| black_box(&pod_3).numeric_identity())
    });

    let pod_10 = Identity {
        namespace: Arc::from("default"),
        pod_name: Arc::from("nginx-abc"),
        service_name: Arc::from(""),
        node_name: Arc::from(""),
        labels: (0..10)
            .map(|i| Arc::from(format!("label-{i}=value-{i}").as_str()))
            .collect::<Vec<_>>()
            .into(),
        workloads: Arc::from(Vec::<Workload>::new()),
    };
    group.bench_function("pod_10_labels", |b| {
        b.iter(|| black_box(&pod_10).numeric_identity())
    });

    let pod_irrelevant = Identity {
        namespace: Arc::from("default"),
        pod_name: Arc::from("nginx-abc"),
        service_name: Arc::from(""),
        node_name: Arc::from(""),
        labels: vec![
            Arc::from("app=nginx"),
            Arc::from("pod-template-hash=abc123"),
            Arc::from("controller-revision-hash=xyz789"),
            Arc::from("tier=frontend"),
        ]
        .into(),
        workloads: Arc::from(Vec::<Workload>::new()),
    };
    group.bench_function("pod_with_irrelevant", |b| {
        b.iter(|| black_box(&pod_irrelevant).numeric_identity())
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_ipcache_get,
    bench_ipcache_get_pair,
    bench_numeric_identity
);
criterion_main!(benches);
