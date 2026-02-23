mod bench_helpers;

use std::net::Ipv4Addr;
use std::sync::{Arc, Barrier};
use std::time::Instant;

use criterion::{BenchmarkId, Criterion, criterion_group, criterion_main};
use retina_core::enricher::enrich_flow;
use retina_core::flow::packet_event_to_flow;
use retina_core::metrics::{ForwardLabels, Metrics};
use retina_core::store::FlowStore;
use tokio::sync::broadcast;

const ITERATIONS_PER_THREAD: usize = 10_000;

fn bench_ipcache_contention(c: &mut Criterion) {
    let mut group = c.benchmark_group("contention/ipcache_read");
    group.sample_size(20);

    for num_threads in [1, 2, 4, 8] {
        let cache = Arc::new(bench_helpers::make_populated_ipcache(5_000));

        group.bench_with_input(
            BenchmarkId::from_parameter(num_threads),
            &num_threads,
            |b, &n| {
                b.iter_custom(|iters| {
                    let mut total = std::time::Duration::ZERO;
                    for _ in 0..iters {
                        let barrier = Arc::new(Barrier::new(n));
                        let handles: Vec<_> = (0..n)
                            .map(|t| {
                                let cache = cache.clone();
                                let barrier = barrier.clone();
                                std::thread::spawn(move || {
                                    let ip1 = std::net::IpAddr::V4(Ipv4Addr::new(
                                        10,
                                        0,
                                        0,
                                        (t as u8) + 1,
                                    ));
                                    let ip2 = std::net::IpAddr::V4(Ipv4Addr::new(
                                        10,
                                        0,
                                        0,
                                        (t as u8) + 2,
                                    ));
                                    barrier.wait();
                                    let start = Instant::now();
                                    for _ in 0..ITERATIONS_PER_THREAD {
                                        let _ = cache.get_pair(&ip1, &ip2);
                                    }
                                    start.elapsed()
                                })
                            })
                            .collect();

                        let max_elapsed = handles
                            .into_iter()
                            .map(|h| h.join().unwrap())
                            .max()
                            .unwrap();
                        total += max_elapsed;
                    }
                    total
                });
            },
        );
    }

    group.finish();
}

fn bench_full_pipeline_contention(c: &mut Criterion) {
    let mut group = c.benchmark_group("contention/full_pipeline");
    group.sample_size(20);

    for num_threads in [1, 2, 4, 8] {
        let cache = Arc::new(bench_helpers::make_populated_ipcache(5_000));
        let metrics = Arc::new(Metrics::new());
        let store = Arc::new(FlowStore::new(131_072));
        let (flow_tx, _rx) = broadcast::channel::<Arc<retina_proto::flow::Flow>>(4096);

        group.bench_with_input(
            BenchmarkId::from_parameter(num_threads),
            &num_threads,
            |b, &n| {
                b.iter_custom(|iters| {
                    let mut total = std::time::Duration::ZERO;
                    for _ in 0..iters {
                        let barrier = Arc::new(Barrier::new(n));
                        let handles: Vec<_> = (0..n)
                            .map(|t| {
                                let cache = cache.clone();
                                let metrics = metrics.clone();
                                let store = store.clone();
                                let tx = flow_tx.clone();
                                let barrier = barrier.clone();
                                std::thread::spawn(move || {
                                    let mut pkt = bench_helpers::make_tcp_ack_event();
                                    pkt.src_ip = u32::from(Ipv4Addr::new(10, 0, 0, (t as u8) + 1));
                                    barrier.wait();
                                    let start = Instant::now();
                                    for _ in 0..ITERATIONS_PER_THREAD {
                                        let mut flow = packet_event_to_flow(&pkt, 0);
                                        enrich_flow(&mut flow, &cache);
                                        let labels = ForwardLabels::from_flow(&flow);
                                        metrics.touch_forward(labels);
                                        let arc = Arc::new(flow);
                                        store.push(arc.clone());
                                        let _ = tx.send(arc);
                                    }
                                    start.elapsed()
                                })
                            })
                            .collect();

                        let max_elapsed = handles
                            .into_iter()
                            .map(|h| h.join().unwrap())
                            .max()
                            .unwrap();
                        total += max_elapsed;
                    }
                    total
                });
            },
        );
    }

    group.finish();
}

criterion_group!(
    benches,
    bench_ipcache_contention,
    bench_full_pipeline_contention
);
criterion_main!(benches);
