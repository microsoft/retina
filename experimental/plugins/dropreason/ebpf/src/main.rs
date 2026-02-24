#![no_std]
#![no_main]

use aya_ebpf::{
    helpers::{bpf_ktime_get_boot_ns, bpf_probe_read_kernel},
    macros::{fexit, map},
    maps::{Array, PerCpuHashMap},
    programs::FExitContext,
};
#[cfg(feature = "ringbuf")]
use aya_ebpf::maps::RingBuf;
#[cfg(not(feature = "ringbuf"))]
use aya_ebpf::maps::PerfEventArray;
use aya_log_ebpf::info;
use dropreason_common::*;

// ── Maps ────────────────────────────────────────────────────────────────────

#[cfg(feature = "ringbuf")]
#[map]
static DROPREASON_EVENTS: RingBuf = RingBuf::with_byte_size(1_048_576, 0); // 1 MiB

#[cfg(not(feature = "ringbuf"))]
#[map]
static DROPREASON_EVENTS: PerfEventArray<DropEvent> = PerfEventArray::new(0);

#[map]
static DROPREASON_METRICS: PerCpuHashMap<DropMetricsKey, DropMetricsValue> =
    PerCpuHashMap::with_max_entries(512, 0);

/// Kernel struct field offsets, populated by the userspace loader from BTF.
/// Each entry is a byte offset (stored as u32). Indexed by `OFFSET_*` constants.
#[map]
static DROPREASON_OFFSETS: Array<u32> = Array::with_max_entries(OFFSET_MAP_SIZE, 0);

// ── Helpers ─────────────────────────────────────────────────────────────────

/// Read a `T` from kernel memory at `ptr + offset`.
///
/// Returns the value on success, or a zeroed `T` on failure (best-effort:
/// we never want to abort an fexit program on a read failure).
#[inline(always)]
unsafe fn read_kernel<T: Copy + Default>(ptr: *const u8, offset: usize) -> T {
    match bpf_probe_read_kernel(ptr.add(offset) as *const T) {
        Ok(v) => v,
        Err(_) => T::default(),
    }
}

/// Read a kernel struct field byte offset from the `DROPREASON_OFFSETS` map.
///
/// The loader populates this map from BTF at startup. Returns the offset on
/// success, or 0 if the map entry is missing (should never happen once loaded).
#[inline(always)]
fn get_offset(index: u32) -> usize {
    match DROPREASON_OFFSETS.get(index) {
        Some(&v) => v as usize,
        None => 0,
    }
}

/// Update the aggregate per-CPU metrics map.
#[inline(always)]
fn update_metrics(reason: DropReason, direction: u8, ret_val: i32, pkt_bytes: u32) {
    let key = DropMetricsKey {
        drop_reason: reason as u8,
        direction,
        _pad: [0; 2],
        return_val: ret_val,
    };
    if let Some(val) = DROPREASON_METRICS.get_ptr_mut(&key) {
        let val = unsafe { &mut *val };
        val.count += 1;
        val.bytes += pkt_bytes as u64;
    } else {
        let val = DropMetricsValue {
            count: 1,
            bytes: pkt_bytes as u64,
        };
        let _ = DROPREASON_METRICS.insert(&key, &val, 0);
    }
}

/// Emit a DropEvent to the ring buffer or perf event array.
#[inline(always)]
fn emit_event(#[allow(unused)] ctx: &FExitContext, event: &DropEvent) {
    #[cfg(feature = "ringbuf")]
    {
        let _ = DROPREASON_EVENTS.output::<DropEvent>(event, 0);
    }
    #[cfg(not(feature = "ringbuf"))]
    {
        DROPREASON_EVENTS.output(ctx, event, 0);
    }
}

/// Extract IP 5-tuple from `struct sk_buff`.
///
/// Reads `head`, `network_header`, `transport_header` from the skb, then
/// parses IP and TCP/UDP headers from the linear data area.
#[inline(always)]
unsafe fn extract_from_skb(skb: *const u8, event: &mut DropEvent) {
    // skb->len
    event.bytes = read_kernel::<u32>(skb, get_offset(OFFSET_SKB_LEN));

    // skb->head (base pointer for header offsets)
    let head: u64 = read_kernel::<u64>(skb, get_offset(OFFSET_SKB_HEAD));

    // skb->network_header (u16 offset from head)
    let nw_off: u16 = read_kernel::<u16>(skb, get_offset(OFFSET_SKB_NETWORK_HEADER));
    // skb->transport_header (u16 offset from head)
    let th_off: u16 = read_kernel::<u16>(skb, get_offset(OFFSET_SKB_TRANSPORT_HEADER));

    if head == 0 {
        return;
    }
    let head = head as *const u8;

    // Read the first 20 bytes of the IP header.
    let ip_hdr: [u8; 20] = match bpf_probe_read_kernel(head.add(nw_off as usize) as *const [u8; 20]) {
        Ok(v) => v,
        Err(_) => return,
    };

    // IP header: protocol @ offset 9, src_ip @ 12, dst_ip @ 16.
    event.proto = ip_hdr[9];
    event.src_ip =
        u32::from_be_bytes([ip_hdr[12], ip_hdr[13], ip_hdr[14], ip_hdr[15]]);
    event.dst_ip =
        u32::from_be_bytes([ip_hdr[16], ip_hdr[17], ip_hdr[18], ip_hdr[19]]);

    // Read the first 4 bytes of L4 header (src_port + dst_port).
    let l4_hdr: [u8; 4] = match bpf_probe_read_kernel(head.add(th_off as usize) as *const [u8; 4]) {
        Ok(v) => v,
        Err(_) => [0u8; 4],
    };

    // TCP (6) and UDP (17) both have src_port and dst_port as the first two
    // big-endian u16 fields.
    if event.proto == 6 || event.proto == 17 {
        event.src_port = u16::from_be_bytes([l4_hdr[0], l4_hdr[1]]);
        event.dst_port = u16::from_be_bytes([l4_hdr[2], l4_hdr[3]]);
    }
}

/// Extract IP 4-tuple from `struct sock` (TCP hooks).
#[inline(always)]
unsafe fn extract_from_sock(sk: *const u8, event: &mut DropEvent) {
    event.proto = 6; // TCP
    event.bytes = 0; // sock-based hooks don't have a packet length

    let saddr: u32 = read_kernel(sk, get_offset(OFFSET_SOCK_SKC_RCV_SADDR));
    let daddr: u32 = read_kernel(sk, get_offset(OFFSET_SOCK_SKC_DADDR));
    let dport_be: u16 = read_kernel(sk, get_offset(OFFSET_SOCK_SKC_DPORT));
    let sport: u16 = read_kernel(sk, get_offset(OFFSET_SOCK_SKC_NUM));

    // saddr/daddr are __be32 — convert to host order for userspace.
    event.src_ip = u32::from_be(saddr);
    event.dst_ip = u32::from_be(daddr);
    event.src_port = sport; // skc_num is already host byte order
    event.dst_port = u16::from_be(dport_be); // skc_dport is network byte order
}

/// Read the netfilter hook number from `struct nf_hook_state` and convert
/// it to a traffic direction.
#[inline(always)]
unsafe fn direction_from_hook_state(state: *const u8) -> u8 {
    if state.is_null() {
        return DIR_UNKNOWN;
    }
    let hook: u8 = read_kernel(state, get_offset(OFFSET_NF_HOOK_STATE_HOOK));
    nf_hook_to_direction(hook as u32)
}

// ── fexit programs ──────────────────────────────────────────────────────────

/// `nf_hook_slow`: iptables / netfilter drop when return value < 0.
///
/// ```c
/// int nf_hook_slow(struct sk_buff *skb, struct nf_hook_state *state,
///                  const struct nf_hook_entries *e, unsigned int s);
/// ```
///
/// fexit args: (skb=0, state=1, e=2, s=3, ret=4)
#[fexit(function = "nf_hook_slow")]
pub fn nf_hook_slow_fexit(ctx: FExitContext) -> i32 {
    match try_nf_hook_slow(&ctx) {
        Ok(()) | Err(()) => 0,
    }
}

#[inline(always)]
fn try_nf_hook_slow(ctx: &FExitContext) -> Result<(), ()> {
    // nf_hook_slow returns 1 on NF_ACCEPT, 0 on NF_DROP, negative errno on error.
    // We want to capture drops (ret <= 0).
    let ret: i32 = ctx.arg(4);
    if ret > 0 {
        return Ok(());
    }

    let skb: *const u8 = ctx.arg(0);
    let state: *const u8 = ctx.arg(1);

    let direction = unsafe { direction_from_hook_state(state) };

    let mut event = DropEvent {
        ts_ns: unsafe { bpf_ktime_get_boot_ns() },
        drop_reason: DropReason::IptableRuleDrop as u8,
        direction,
        return_val: ret as i8,
        ..DropEvent::default()
    };

    if !skb.is_null() {
        unsafe { extract_from_skb(skb, &mut event) };
    }

    update_metrics(DropReason::IptableRuleDrop, direction, ret, event.bytes);
    emit_event(ctx, &event);
    Ok(())
}

/// `nf_nat_inet_fn`: NAT drop when return value == NF_DROP (0).
///
/// ```c
/// unsigned int nf_nat_inet_fn(void *priv, struct sk_buff *skb,
///                             const struct nf_hook_state *state);
/// ```
///
/// fexit args: (priv=0, skb=1, state=2, ret=3)
#[fexit(function = "nf_nat_inet_fn")]
pub fn nf_nat_inet_fn_fexit(ctx: FExitContext) -> i32 {
    match try_nf_nat_inet_fn(&ctx) {
        Ok(()) | Err(()) => 0,
    }
}

#[inline(always)]
fn try_nf_nat_inet_fn(ctx: &FExitContext) -> Result<(), ()> {
    let ret: u32 = ctx.arg(3);
    if ret != NF_DROP {
        return Ok(());
    }

    let skb: *const u8 = ctx.arg(1);
    let state: *const u8 = ctx.arg(2);

    let direction = unsafe { direction_from_hook_state(state) };

    let mut event = DropEvent {
        ts_ns: unsafe { bpf_ktime_get_boot_ns() },
        drop_reason: DropReason::IptableNatDrop as u8,
        direction,
        return_val: 0,
        ..DropEvent::default()
    };

    if !skb.is_null() {
        unsafe { extract_from_skb(skb, &mut event) };
    }

    update_metrics(DropReason::IptableNatDrop, direction, ret as i32, event.bytes);
    emit_event(ctx, &event);
    Ok(())
}

/// `__nf_conntrack_confirm`: conntrack drop when return value == NF_DROP (0).
///
/// ```c
/// unsigned int __nf_conntrack_confirm(struct sk_buff *skb);
/// ```
///
/// fexit args: (skb=0, ret=1)
#[fexit(function = "__nf_conntrack_confirm")]
pub fn nf_conntrack_confirm_fexit(ctx: FExitContext) -> i32 {
    match try_nf_conntrack_confirm(&ctx) {
        Ok(()) | Err(()) => 0,
    }
}

#[inline(always)]
fn try_nf_conntrack_confirm(ctx: &FExitContext) -> Result<(), ()> {
    let ret: u32 = ctx.arg(1);
    if ret != NF_DROP {
        return Ok(());
    }

    let skb: *const u8 = ctx.arg(0);

    let mut event = DropEvent {
        ts_ns: unsafe { bpf_ktime_get_boot_ns() },
        drop_reason: DropReason::ConntrackDrop as u8,
        direction: DIR_UNKNOWN,
        return_val: 0,
        ..DropEvent::default()
    };

    if !skb.is_null() {
        unsafe { extract_from_skb(skb, &mut event) };
    }

    update_metrics(DropReason::ConntrackDrop, DIR_UNKNOWN, ret as i32, event.bytes);
    emit_event(ctx, &event);
    Ok(())
}

/// `tcp_v4_connect`: TCP connect failure when return value != 0.
///
/// ```c
/// int tcp_v4_connect(struct sock *sk, struct sockaddr *uaddr, int addr_len);
/// ```
///
/// fexit args: (sk=0, uaddr=1, addr_len=2, ret=3)
#[fexit(function = "tcp_v4_connect")]
pub fn tcp_v4_connect_fexit(ctx: FExitContext) -> i32 {
    match try_tcp_v4_connect(&ctx) {
        Ok(()) | Err(()) => 0,
    }
}

#[inline(always)]
fn try_tcp_v4_connect(ctx: &FExitContext) -> Result<(), ()> {
    let ret: i32 = ctx.arg(3);
    if ret == 0 {
        return Ok(());
    }

    let sk: *const u8 = ctx.arg(0);
    let uaddr: *const u8 = ctx.arg(1);

    let mut event = DropEvent {
        ts_ns: unsafe { bpf_ktime_get_boot_ns() },
        drop_reason: DropReason::TcpConnectDrop as u8,
        direction: DIR_EGRESS,
        return_val: ret as i8,
        ..DropEvent::default()
    };

    if !sk.is_null() {
        unsafe { extract_from_sock(sk, &mut event) };
    }

    // For early connect failures the sock may not have addresses yet.
    // Fall back to reading the intended destination from the sockaddr_in
    // argument (stable UAPI layout, no BTF needed).
    if event.dst_ip == 0 && !uaddr.is_null() {
        unsafe {
            let addr_be: u32 = read_kernel(uaddr, SOCKADDR_IN_ADDR);
            let port_be: u16 = read_kernel(uaddr, SOCKADDR_IN_PORT);
            event.dst_ip = u32::from_be(addr_be);
            event.dst_port = u16::from_be(port_be);
            event.proto = 6; // TCP
        }
    }

    info!(ctx, "tcp_v4_connect DROP ret={} src={}:{} dst={}:{} proto={}",
        ret, event.src_ip, event.src_port, event.dst_ip, event.dst_port, event.proto);

    update_metrics(DropReason::TcpConnectDrop, DIR_EGRESS, ret, event.bytes);
    emit_event(ctx, &event);
    Ok(())
}

/// `inet_csk_accept`: TCP accept failure when returned sock pointer is NULL.
///
/// ```c
/// struct sock *inet_csk_accept(struct sock *sk, int flags, int *err, bool kern);
/// ```
///
/// fexit args: (sk=0, flags=1, err=2, kern=3, retsk=4)
#[fexit(function = "inet_csk_accept")]
pub fn inet_csk_accept_fexit(ctx: FExitContext) -> i32 {
    match try_inet_csk_accept(&ctx) {
        Ok(()) | Err(()) => 0,
    }
}

#[inline(always)]
fn try_inet_csk_accept(ctx: &FExitContext) -> Result<(), ()> {
    let retsk: u64 = ctx.arg(4);
    if retsk != 0 {
        return Ok(()); // successful accept
    }

    // Read the error pointer (arg 2 = int *err) and dereference it.
    // Non-blocking accept with empty queue sets *err = -EAGAIN — not a real drop.
    let err_ptr: *const i32 = ctx.arg(2);
    let err_val: i32 = if err_ptr.is_null() {
        0
    } else {
        unsafe {
            match bpf_probe_read_kernel(err_ptr) {
                Ok(v) => v,
                Err(_) => 0,
            }
        }
    };

    // Skip non-drops: no error, or EAGAIN from non-blocking accept.
    if err_val >= 0 || err_val == -(EAGAIN as i32) {
        return Ok(());
    }

    // Use the listening socket (first arg) for local IP/port info.
    let sk: *const u8 = ctx.arg(0);

    let mut event = DropEvent {
        ts_ns: unsafe { bpf_ktime_get_boot_ns() },
        drop_reason: DropReason::TcpAcceptDrop as u8,
        direction: DIR_INGRESS,
        return_val: err_val as i8,
        ..DropEvent::default()
    };

    if !sk.is_null() {
        unsafe { extract_from_sock(sk, &mut event) };
    }

    info!(ctx, "inet_csk_accept DROP err={} src={}:{} dst={}:{}",
        err_val, event.src_ip, event.src_port, event.dst_ip, event.dst_port);

    update_metrics(DropReason::TcpAcceptDrop, DIR_INGRESS, err_val, event.bytes);
    emit_event(ctx, &event);
    Ok(())
}

// ── Panic handler (required for no_std) ─────────────────────────────────────

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    unsafe { core::hint::unreachable_unchecked() }
}
