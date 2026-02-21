#![no_std]
#![no_main]

use aya_ebpf::{
    bindings::TC_ACT_UNSPEC,
    macros::{classifier, map},
    maps::{LruHashMap, PerfEventArray},
    programs::TcContext,
};
use aya_log_ebpf::trace;
use network_types::{
    eth::{EthHdr, EtherType},
    ip::{IpProto, Ipv4Hdr},
    tcp::TcpHdr,
    udp::UdpHdr,
};
use retina_common::*;

mod conntrack;

#[map]
static EVENTS: PerfEventArray<PacketEvent> = PerfEventArray::new(0);

#[map]
static CONNTRACK: LruHashMap<CtV4Key, CtEntry> = LruHashMap::with_max_entries(CT_MAP_SIZE, 0);

/// Core packet parsing and conntrack processing.
#[inline(always)]
fn parse(ctx: &TcContext, obs_point: u8) -> i32 {
    match try_parse(ctx, obs_point) {
        Ok(ret) => ret,
        Err(_) => TC_ACT_UNSPEC,
    }
}

#[inline(always)]
fn try_parse(ctx: &TcContext, obs_point: u8) -> Result<i32, ()> {
    let ts_ns = unsafe { aya_ebpf::helpers::bpf_ktime_get_boot_ns() };
    let skb_len = ctx.len();

    // Parse Ethernet header.
    let eth_hdr: EthHdr = ctx.load(0).map_err(|_| ())?;
    let ether_type = { eth_hdr.ether_type };
    if ether_type != u16::from(EtherType::Ipv4) {
        return Ok(TC_ACT_UNSPEC);
    }

    // Parse IPv4 header.
    let ipv4_hdr: Ipv4Hdr = ctx.load(EthHdr::LEN).map_err(|_| ())?;
    let src_ip = ipv4_hdr.src_addr;
    let dst_ip = ipv4_hdr.dst_addr;
    let proto = ipv4_hdr.proto;

    let mut pkt: PacketEvent = unsafe { core::mem::zeroed() };
    pkt.ts_ns = ts_ns;
    pkt.bytes = skb_len;
    pkt.src_ip = u32::from_be_bytes(src_ip);
    pkt.dst_ip = u32::from_be_bytes(dst_ip);
    pkt.proto = proto as u8;
    pkt.observation_point = obs_point;

    let ip_hdr_len = EthHdr::LEN + Ipv4Hdr::LEN;

    match proto {
        IpProto::Tcp => {
            let tcp_hdr: TcpHdr = ctx.load(ip_hdr_len).map_err(|_| ())?;
            pkt.src_port = u16::from_be_bytes(tcp_hdr.source);
            pkt.dst_port = u16::from_be_bytes(tcp_hdr.dest);

            // Extract TCP flags from the combined field.
            // TcpHdr stores flags in a u16 bitfield at bytes 12-13.
            // We read the raw flags byte at offset 13 from TCP header start.
            let flags_offset = ip_hdr_len + 13;
            let flags_byte: u8 = ctx.load(flags_offset).map_err(|_| ())?;
            // Also check byte 12 for NS flag (bit 0 of high nibble).
            let doff_byte: u8 = ctx.load(ip_hdr_len + 12).map_err(|_| ())?;

            let mut flags: u16 = flags_byte as u16;
            if (doff_byte & 0x01) != 0 {
                flags |= TCP_NS;
            }
            pkt.flags = flags;
        }
        IpProto::Udp => {
            let udp_hdr: UdpHdr = ctx.load(ip_hdr_len).map_err(|_| ())?;
            pkt.src_port = u16::from_be_bytes(udp_hdr.src);
            pkt.dst_port = u16::from_be_bytes(udp_hdr.dst);
        }
        _ => {
            return Ok(TC_ACT_UNSPEC);
        }
    }

    // Process through conntrack.
    let report = conntrack::ct_process_packet(&CONNTRACK, &mut pkt, obs_point);

    if report.report {
        pkt.previously_observed_packets = report.previously_observed_packets;
        pkt.previously_observed_bytes = report.previously_observed_bytes;
        pkt.prev_flags = report.previously_observed_flags;

        trace!(
            ctx,
            "pkt src={:i} dst={:i} proto={} bytes={}",
            u32::from_be_bytes(src_ip),
            u32::from_be_bytes(dst_ip),
            proto as u8,
            skb_len,
        );

        EVENTS.output(ctx, &pkt, 0);
    }

    Ok(TC_ACT_UNSPEC)
}

#[classifier]
pub fn endpoint_ingress(ctx: TcContext) -> i32 {
    parse(&ctx, OBS_FROM_ENDPOINT)
}

#[classifier]
pub fn endpoint_egress(ctx: TcContext) -> i32 {
    parse(&ctx, OBS_TO_ENDPOINT)
}

#[classifier]
pub fn host_ingress(ctx: TcContext) -> i32 {
    parse(&ctx, OBS_FROM_NETWORK)
}

#[classifier]
pub fn host_egress(ctx: TcContext) -> i32 {
    parse(&ctx, OBS_TO_NETWORK)
}

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    unsafe { core::hint::unreachable_unchecked() }
}
