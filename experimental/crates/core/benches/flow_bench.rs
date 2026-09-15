mod bench_helpers;

use criterion::{Criterion, black_box, criterion_group, criterion_main};
use retina_core::flow::{make_extensions, packet_event_to_flow, tcp_flags_summary};
use retina_proto::flow::TcpFlags;

fn bench_packet_event_to_flow(c: &mut Criterion) {
    let mut group = c.benchmark_group("packet_event_to_flow");

    let tcp_syn = bench_helpers::make_tcp_syn_event();
    group.bench_function("tcp_syn", |b| {
        b.iter(|| packet_event_to_flow(black_box(&tcp_syn), black_box(0)))
    });

    let tcp_syn_ack = bench_helpers::make_tcp_syn_ack_event();
    group.bench_function("tcp_syn_ack", |b| {
        b.iter(|| packet_event_to_flow(black_box(&tcp_syn_ack), black_box(0)))
    });

    let tcp_ack = bench_helpers::make_tcp_ack_event();
    group.bench_function("tcp_ack", |b| {
        b.iter(|| packet_event_to_flow(black_box(&tcp_ack), black_box(0)))
    });

    let udp = bench_helpers::make_udp_event();
    group.bench_function("udp", |b| {
        b.iter(|| packet_event_to_flow(black_box(&udp), black_box(0)))
    });

    group.finish();
}

fn bench_tcp_flags_summary(c: &mut Criterion) {
    let mut group = c.benchmark_group("tcp_flags_summary");

    let syn = TcpFlags {
        syn: true,
        ..Default::default()
    };
    group.bench_function("syn", |b| b.iter(|| tcp_flags_summary(black_box(&syn))));

    let syn_ack = TcpFlags {
        syn: true,
        ack: true,
        ..Default::default()
    };
    group.bench_function("syn_ack", |b| {
        b.iter(|| tcp_flags_summary(black_box(&syn_ack)))
    });

    let ack = TcpFlags {
        ack: true,
        ..Default::default()
    };
    group.bench_function("ack", |b| b.iter(|| tcp_flags_summary(black_box(&ack))));

    let fin_rst = TcpFlags {
        fin: true,
        rst: true,
        ..Default::default()
    };
    group.bench_function("fin_rst", |b| {
        b.iter(|| tcp_flags_summary(black_box(&fin_rst)))
    });

    group.finish();
}

fn bench_make_extensions(c: &mut Criterion) {
    let mut group = c.benchmark_group("make_extensions");

    group.bench_function("nonzero", |b| b.iter(|| make_extensions(black_box(1500))));

    group.bench_function("zero", |b| b.iter(|| make_extensions(black_box(0))));

    group.finish();
}

criterion_group!(
    benches,
    bench_packet_event_to_flow,
    bench_tcp_flags_summary,
    bench_make_extensions
);
criterion_main!(benches);
