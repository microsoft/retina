use anyhow::Context as _;
use aya::maps::{Array, MapData, PerCpuHashMap, PerfEventArray, RingBuf};
use aya::programs::FExit;
use aya::{Btf, Ebpf, EbpfLoader};
use btf_rs::Type;
use dropreason_common::*;
use tracing::{info, warn};

/// Event source abstraction: ring buffer (Linux 5.8+) or perf event array.
pub enum EventSource {
    Perf(PerfEventArray<MapData>),
    Ring(RingBuf<MapData>),
}

pub type EbpfHandles = (
    Ebpf,
    EventSource,
    PerCpuHashMap<MapData, DropMetricsKey, DropMetricsValue>,
);

/// Force 8-byte alignment on embedded byte data so the `object` crate's ELF
/// parser can cast the pointer to `Elf64_Ehdr` without misalignment.
/// (`include_bytes!` only guarantees 1-byte alignment.)
#[repr(C, align(8))]
struct Align8<Bytes: ?Sized> {
    bytes: Bytes,
}

/// eBPF object (perf variant) embedded at compile time.
static EBPF_OBJ_PERF: &Align8<[u8]> = &Align8 {
    bytes: *include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../ebpf/target/bpfel-unknown-none/release/dropreason-ebpf"
    )),
};

/// eBPF object (ringbuf variant) embedded at compile time.
static EBPF_OBJ_RINGBUF: &Align8<[u8]> = &Align8 {
    bytes: *include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../ebpf/target/bpfel-unknown-none/release/dropreason-ebpf-ringbuf"
    )),
};

/// Check if the running kernel supports BPF ring buffers (>= 5.8).
fn kernel_supports_ringbuf() -> bool {
    unsafe {
        let mut utsname: libc::utsname = core::mem::zeroed();
        if libc::uname(&mut utsname) != 0 {
            return false;
        }
        let release = core::ffi::CStr::from_ptr(utsname.release.as_ptr());
        let release = release.to_string_lossy();
        let mut parts = release.split('.');
        let major: u32 = parts.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        let minor: u32 = parts.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        major > 5 || (major == 5 && minor >= 8)
    }
}

/// Names of the 5 fexit programs in the eBPF object.
///
/// Some kernel functions may not exist (e.g. `__nf_conntrack_confirm` on
/// kernels without the conntrack module). The loader will skip missing
/// programs and continue.
const FEXIT_PROGRAMS: &[&str] = &[
    "nf_hook_slow_fexit",
    "nf_nat_inet_fn_fexit",
    "nf_conntrack_confirm_fexit",
    "tcp_v4_connect_fexit",
    "inet_csk_accept_fexit",
];

// ── BTF offset resolution ─────────────────────────────────────────────────

/// Resolved kernel struct field byte offsets.
struct KernelOffsets {
    skb_len: u32,
    skb_transport_header: u32,
    skb_network_header: u32,
    skb_head: u32,
    sock_skc_daddr: u32,
    sock_skc_rcv_saddr: u32,
    sock_skc_dport: u32,
    sock_skc_num: u32,
    nf_hook_state_hook: u32,
}

/// Find a named field's byte offset within a BTF struct, recursing into
/// anonymous structs/unions. Returns `None` if the field is not found.
fn find_member_offset(
    btf: &btf_rs::Btf,
    members: &[btf_rs::Member],
    field_name: &str,
    base_bits: u32,
) -> Option<u32> {
    for member in members {
        let name = btf.resolve_name(member).unwrap_or_else(|_| String::new());
        let bit_off = base_bits + member.bit_offset();

        if name == field_name {
            return Some(bit_off / 8);
        }

        // Anonymous struct/union: search inside without consuming the target name.
        if name.is_empty()
            && let Ok(inner) = btf.resolve_chained_type(member)
        {
            let sub_members = match &inner {
                Type::Struct(s) => &s.members,
                Type::Union(u) => &u.members,
                _ => continue,
            };
            if let Some(offset) = find_member_offset(btf, sub_members, field_name, bit_off) {
                return Some(offset);
            }
        }
    }
    None
}

/// Resolve a struct field's byte offset from kernel BTF.
fn resolve_field(btf: &btf_rs::Btf, struct_name: &str, field_name: &str) -> anyhow::Result<u32> {
    let types = btf
        .resolve_types_by_name(struct_name)
        .with_context(|| format!("BTF struct '{struct_name}' not found"))?;

    for ty in &types {
        if let Type::Struct(s) = ty
            && let Some(offset) = find_member_offset(btf, &s.members, field_name, 0)
        {
            return Ok(offset);
        }
    }

    anyhow::bail!("field '{field_name}' not found in BTF struct '{struct_name}'")
}

/// Parse kernel BTF and resolve all needed struct field offsets.
fn resolve_kernel_offsets() -> anyhow::Result<KernelOffsets> {
    let btf = btf_rs::Btf::from_file("/sys/kernel/btf/vmlinux")
        .context("failed to parse /sys/kernel/btf/vmlinux")?;

    Ok(KernelOffsets {
        skb_len: resolve_field(&btf, "sk_buff", "len")?,
        skb_transport_header: resolve_field(&btf, "sk_buff", "transport_header")?,
        skb_network_header: resolve_field(&btf, "sk_buff", "network_header")?,
        skb_head: resolve_field(&btf, "sk_buff", "head")?,
        sock_skc_daddr: resolve_field(&btf, "sock_common", "skc_daddr")?,
        sock_skc_rcv_saddr: resolve_field(&btf, "sock_common", "skc_rcv_saddr")?,
        sock_skc_dport: resolve_field(&btf, "sock_common", "skc_dport")?,
        sock_skc_num: resolve_field(&btf, "sock_common", "skc_num")?,
        nf_hook_state_hook: resolve_field(&btf, "nf_hook_state", "hook")?,
    })
}

/// Write resolved offsets into the eBPF `DROPREASON_OFFSETS` array map.
fn populate_offsets_map(ebpf: &mut Ebpf, offsets: &KernelOffsets) -> anyhow::Result<()> {
    let mut map: Array<_, u32> = Array::try_from(
        ebpf.map_mut("DROPREASON_OFFSETS")
            .context("eBPF map 'DROPREASON_OFFSETS' not found")?,
    )?;

    map.set(OFFSET_SKB_LEN, offsets.skb_len, 0)?;
    map.set(OFFSET_SKB_TRANSPORT_HEADER, offsets.skb_transport_header, 0)?;
    map.set(OFFSET_SKB_NETWORK_HEADER, offsets.skb_network_header, 0)?;
    map.set(OFFSET_SKB_HEAD, offsets.skb_head, 0)?;
    map.set(OFFSET_SOCK_SKC_DADDR, offsets.sock_skc_daddr, 0)?;
    map.set(OFFSET_SOCK_SKC_RCV_SADDR, offsets.sock_skc_rcv_saddr, 0)?;
    map.set(OFFSET_SOCK_SKC_DPORT, offsets.sock_skc_dport, 0)?;
    map.set(OFFSET_SOCK_SKC_NUM, offsets.sock_skc_num, 0)?;
    map.set(OFFSET_NF_HOOK_STATE_HOOK, offsets.nf_hook_state_hook, 0)?;

    info!(
        skb_len = offsets.skb_len,
        skb_transport_header = offsets.skb_transport_header,
        skb_network_header = offsets.skb_network_header,
        skb_head = offsets.skb_head,
        sock_skc_daddr = offsets.sock_skc_daddr,
        sock_skc_rcv_saddr = offsets.sock_skc_rcv_saddr,
        sock_skc_dport = offsets.sock_skc_dport,
        sock_skc_num = offsets.sock_skc_num,
        nf_hook_state_hook = offsets.nf_hook_state_hook,
        "dropreason: resolved kernel struct offsets from BTF"
    );

    Ok(())
}

// ── eBPF loading ─────────────────────────────────────────────────────────────

/// Load all fexit programs and attach them to their target kernel functions.
///
/// Returns the Ebpf handle (owns the programs), an event source for reading
/// drop events, and the per-CPU metrics map handle.
pub fn load_and_attach(ring_buffer_size: u32) -> anyhow::Result<EbpfHandles> {
    let use_ringbuf = kernel_supports_ringbuf();
    info!(use_ringbuf, "dropreason: selecting event buffer type");

    let mut ebpf = if use_ringbuf {
        info!(ring_buffer_size, "dropreason: loading ringbuf variant");
        EbpfLoader::new()
            .map_max_entries("DROPREASON_EVENTS", ring_buffer_size)
            .load(&EBPF_OBJ_RINGBUF.bytes)?
    } else {
        info!("dropreason: loading perf variant");
        Ebpf::load(&EBPF_OBJ_PERF.bytes)?
    };

    // Resolve kernel struct field offsets from BTF and populate the eBPF config map.
    let offsets = resolve_kernel_offsets()
        .context("failed to resolve kernel struct offsets from BTF")?;
    populate_offsets_map(&mut ebpf, &offsets)
        .context("failed to populate DROPREASON_OFFSETS map")?;

    let btf = Btf::from_sys_fs().context("failed to load kernel BTF from /sys/kernel/btf/vmlinux")?;

    // Load and attach all fexit programs.
    let mut attached = 0;
    for name in FEXIT_PROGRAMS {
        match ebpf.program_mut(name) {
            Some(prog) => {
                let fexit: &mut FExit = prog
                    .try_into()
                    .with_context(|| format!("program '{name}' is not an FExit"))?;

                // The kernel function name is derived from the eBPF program name
                // by stripping the `_fexit` suffix.
                let fn_name = name
                    .strip_suffix("_fexit")
                    .unwrap_or(name);

                match fexit.load(fn_name, &btf) {
                    Ok(()) => {}
                    Err(e) => {
                        warn!(
                            "dropreason: failed to load fexit for '{fn_name}': {e} \
                             (kernel function may not exist)"
                        );
                        continue;
                    }
                }

                match fexit.attach() {
                    Ok(_) => {
                        info!("dropreason: attached fexit to {fn_name}");
                        attached += 1;
                    }
                    Err(e) => {
                        warn!("dropreason: failed to attach fexit for '{fn_name}': {e}");
                    }
                }
            }
            None => {
                warn!("dropreason: program '{name}' not found in eBPF object, skipping");
            }
        }
    }

    if attached == 0 {
        anyhow::bail!("dropreason: no fexit programs could be attached");
    }
    info!(attached, "dropreason: fexit programs attached");

    // Extract maps (take ownership for 'static lifetime).
    let event_source = if use_ringbuf {
        EventSource::Ring(RingBuf::try_from(
            ebpf.take_map("DROPREASON_EVENTS")
                .context("map 'DROPREASON_EVENTS' not found")?,
        )?)
    } else {
        EventSource::Perf(PerfEventArray::try_from(
            ebpf.take_map("DROPREASON_EVENTS")
                .context("map 'DROPREASON_EVENTS' not found")?,
        )?)
    };

    let metrics_map = PerCpuHashMap::try_from(
        ebpf.take_map("DROPREASON_METRICS")
            .context("map 'DROPREASON_METRICS' not found")?,
    )?;

    Ok((ebpf, event_source, metrics_map))
}
