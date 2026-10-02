use std::collections::{HashMap, HashSet};

use anyhow::Context as _;
use aya::maps::{Array, MapData, PerCpuArray, PerCpuHashMap, PerfEventArray, RingBuf};
use aya::programs::{BtfTracePoint, FExit};
use aya::{Btf, Ebpf, EbpfLoader};
use btf_rs::Type;
use dropreason_common::{
    DropMetricsKey, DropMetricsValue, OFFSET_NF_HOOK_STATE_HOOK, OFFSET_SKB_HEAD, OFFSET_SKB_LEN,
    OFFSET_SKB_NETWORK_HEADER, OFFSET_SKB_PKT_TYPE, OFFSET_SKB_TRANSPORT_HEADER,
    OFFSET_SOCK_SKC_DADDR, OFFSET_SOCK_SKC_DPORT, OFFSET_SOCK_SKC_NUM, OFFSET_SOCK_SKC_RCV_SADDR,
};
use retina_core::ebpf::{Align8, kernel_supports_ringbuf};
use tracing::{info, warn};

/// Event source abstraction: ring buffer (Linux 5.8+) or perf event array.
pub(crate) enum EventSource {
    Perf(PerfEventArray<MapData>),
    Ring(RingBuf<MapData>),
}

/// Loaded eBPF state returned by [`load_and_attach`].
pub(crate) struct EbpfHandles {
    pub(crate) ebpf: Ebpf,
    pub(crate) event_source: EventSource,
    pub(crate) metrics_map: PerCpuHashMap<MapData, DropMetricsKey, DropMetricsValue>,
    pub(crate) kernel_drop_reasons: HashMap<u32, String>,
    pub(crate) suppressed_reasons: HashSet<String>,
    /// Per-CPU counter of ring buffer output failures (ringbuf mode only).
    pub(crate) ring_lost_map: Option<PerCpuArray<MapData, u64>>,
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

/// BTF tracepoint programs: (eBPF program name, kernel tracepoint name).
///
/// These require kernel >= 5.5 (BTF tracepoint support). Individual
/// tracepoints may not exist on older kernels — the loader skips them.
const BTF_TP_PROGRAMS: &[(&str, &str)] = &[
    ("kfree_skb_tp", "kfree_skb"),
    ("tcp_retransmit_skb_tp", "tcp_retransmit_skb"),
    ("tcp_send_reset_tp", "tcp_send_reset"),
    ("tcp_receive_reset_tp", "tcp_receive_reset"),
];

// ── BTF offset resolution ─────────────────────────────────────────────────

/// Resolved kernel struct field byte offsets.
struct KernelOffsets {
    skb_len: u32,
    skb_transport_header: u32,
    skb_network_header: u32,
    skb_head: u32,
    skb_pkt_type: u32,
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
        skb_pkt_type: resolve_field(&btf, "sk_buff", "pkt_type")?,
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
    map.set(OFFSET_SKB_PKT_TYPE, offsets.skb_pkt_type, 0)?;
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
        skb_pkt_type = offsets.skb_pkt_type,
        sock_skc_daddr = offsets.sock_skc_daddr,
        sock_skc_rcv_saddr = offsets.sock_skc_rcv_saddr,
        sock_skc_dport = offsets.sock_skc_dport,
        sock_skc_num = offsets.sock_skc_num,
        nf_hook_state_hook = offsets.nf_hook_state_hook,
        "dropreason: resolved kernel struct offsets from BTF"
    );

    Ok(())
}

// ── Kernel drop reason resolution ─────────────────────────────────────────────

/// Resolve `enum skb_drop_reason` variants from kernel BTF.
///
/// Returns a map from enum discriminant (u32) to a shortened variant name
/// (e.g. `"TCP_CSUM"` instead of `"SKB_DROP_REASON_TCP_CSUM"`).
/// Returns an empty map on pre-5.17 kernels where the enum doesn't exist.
fn resolve_kernel_drop_reasons() -> HashMap<u32, String> {
    let btf = match btf_rs::Btf::from_file("/sys/kernel/btf/vmlinux") {
        Ok(btf) => btf,
        Err(e) => {
            warn!("dropreason: cannot parse BTF for drop reason resolution: {e}");
            return HashMap::new();
        }
    };

    let Ok(types) = btf.resolve_types_by_name("skb_drop_reason") else {
        info!("dropreason: enum 'skb_drop_reason' not found in BTF (kernel < 5.17?)");
        return HashMap::new();
    };

    let mut map = HashMap::new();
    for ty in &types {
        if let Type::Enum(e) = ty {
            for member in &e.members {
                let name = btf.resolve_name(member).unwrap_or_else(|_| String::new());
                if !name.is_empty() {
                    // Strip the common "SKB_DROP_REASON_" prefix for cleaner labels.
                    let short = name
                        .strip_prefix("SKB_DROP_REASON_")
                        .unwrap_or(&name)
                        .to_string();
                    map.insert(member.val(), short);
                }
            }
            break;
        }
    }

    info!(
        count = map.len(),
        "dropreason: resolved kernel drop reason enum from BTF"
    );
    map
}

// ── eBPF loading ─────────────────────────────────────────────────────────────

/// Load all eBPF programs (fexit + BTF tracepoints) and attach them.
///
/// Returns the Ebpf handle (owns the programs), an event source for reading
/// drop events, the per-CPU metrics map handle, and a map of kernel drop
/// reason enum values to human-readable names.
pub(crate) fn load_and_attach(
    ring_buffer_size: u32,
    suppressed_reasons: &HashSet<String>,
) -> anyhow::Result<EbpfHandles> {
    let use_ringbuf = kernel_supports_ringbuf();
    info!(use_ringbuf, "dropreason: selecting event buffer type");

    let mut ebpf = if use_ringbuf {
        info!(ring_buffer_size, "dropreason: loading ringbuf variant");
        EbpfLoader::new()
            .map_max_entries("DROPREASON_EVENTS", ring_buffer_size)
            .load(&EBPF_OBJ_RINGBUF.bytes)
            .context("failed to load dropreason ringbuf eBPF variant")?
    } else {
        info!("dropreason: loading perf variant");
        Ebpf::load(&EBPF_OBJ_PERF.bytes).context("failed to load dropreason perf eBPF variant")?
    };

    // Resolve kernel struct field offsets from BTF and populate the eBPF config map.
    let offsets =
        resolve_kernel_offsets().context("failed to resolve kernel struct offsets from BTF")?;
    populate_offsets_map(&mut ebpf, &offsets)
        .context("failed to populate DROPREASON_OFFSETS map")?;

    let btf =
        Btf::from_sys_fs().context("failed to load kernel BTF from /sys/kernel/btf/vmlinux")?;

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
                let fn_name = name.strip_suffix("_fexit").unwrap_or(name);

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

    // Load and attach BTF tracepoint programs.
    for (prog_name, tp_name) in BTF_TP_PROGRAMS {
        match ebpf.program_mut(prog_name) {
            Some(prog) => {
                let tp: &mut BtfTracePoint = match prog.try_into() {
                    Ok(tp) => tp,
                    Err(e) => {
                        warn!("dropreason: program '{prog_name}' is not a BtfTracePoint: {e}");
                        continue;
                    }
                };

                match tp.load(tp_name, &btf) {
                    Ok(()) => {}
                    Err(e) => {
                        warn!(
                            "dropreason: failed to load btf_tracepoint for '{tp_name}': {e} \
                             (tracepoint may not exist on this kernel)"
                        );
                        continue;
                    }
                }

                match tp.attach() {
                    Ok(_) => {
                        info!("dropreason: attached btf_tracepoint to {tp_name}");
                        attached += 1;
                    }
                    Err(e) => {
                        warn!("dropreason: failed to attach btf_tracepoint for '{tp_name}': {e}");
                    }
                }
            }
            None => {
                warn!("dropreason: program '{prog_name}' not found in eBPF object, skipping");
            }
        }
    }

    if attached == 0 {
        anyhow::bail!("dropreason: no eBPF programs could be attached");
    }
    info!(attached, "dropreason: eBPF programs attached");

    // Resolve kernel drop reason enum for kfree_skb flow labeling.
    let kernel_drop_reasons = resolve_kernel_drop_reasons();

    // Populate the eBPF suppress set from the ConfigMap filter config.
    // This resolves reason names to kernel enum values via the BTF-resolved map.
    {
        let mut suppress_map = aya::maps::HashMap::<_, u32, u8>::try_from(
            ebpf.map_mut("DROPREASON_SUPPRESS")
                .context("eBPF map 'DROPREASON_SUPPRESS' not found")?,
        )?;
        let mut suppressed_count = 0u32;
        for (&value, name) in &kernel_drop_reasons {
            if suppressed_reasons.contains(name.as_str()) {
                suppress_map.insert(value, 1u8, 0)?;
                suppressed_count += 1;
            }
        }
        if suppressed_count > 0 {
            info!(
                suppressed_count,
                "dropreason: populated kernel drop reason suppress set"
            );
        }
    }

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

    // Best-effort: only present in the ringbuf variant.
    let ring_lost_map = ebpf
        .take_map("DROPREASON_RING_LOST")
        .and_then(|m| PerCpuArray::<_, u64>::try_from(m).ok());

    Ok(EbpfHandles {
        ebpf,
        event_source,
        metrics_map,
        kernel_drop_reasons,
        suppressed_reasons: suppressed_reasons.clone(),
        ring_lost_map,
    })
}
