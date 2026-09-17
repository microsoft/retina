use anyhow::Context as _;
use aya::{
    Ebpf, EbpfLoader,
    maps::{Array, HashMap, MapData, PerfEventArray, RingBuf},
    programs::{
        LinkOrder, SchedClassifier,
        tc::{NlOptions, SchedClassifierLink, TcAttachOptions, TcAttachType},
    },
};
use retina_common::{CtEntry, CtV4Key};
use retina_core::ebpf::{Align8, kernel_supports_ringbuf};
use tracing::info;

/// Event source abstraction: ring buffer (Linux 5.8+) or perf event array.
pub(crate) enum EventSource {
    Perf(PerfEventArray<MapData>),
    Ring(RingBuf<MapData>),
}

pub(crate) type EbpfHandles = (Ebpf, EventSource, HashMap<MapData, CtV4Key, CtEntry>);

/// eBPF object (perf variant) embedded at compile time.
/// Requires `cargo xtask build-ebpf --release` to run first.
static EBPF_OBJ_PERF: &Align8<[u8]> = &Align8 {
    bytes: *include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../ebpf/target/bpfel-unknown-none/release/packetparser-ebpf"
    )),
};

/// eBPF object (ringbuf variant) embedded at compile time.
static EBPF_OBJ_RINGBUF: &Align8<[u8]> = &Align8 {
    bytes: *include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../ebpf/target/bpfel-unknown-none/release/packetparser-ebpf-ringbuf"
    )),
};

/// Attach a TC classifier, preferring TCX with head-of-chain ordering so Retina
/// runs before any other TC programs on the interface. Falls back to legacy TC
/// on kernels < 6.6.
///
/// Our eBPF programs return `TC_ACT_UNSPEC`, which maps to `TCX_NEXT` in TCX
/// (continues the chain) and `continue` in legacy `cls_bpf` (next filter runs).
/// This lets Retina passively observe every packet without affecting the verdict
/// of subsequent programs.
fn attach_tc(
    prog: &mut SchedClassifier,
    iface: &str,
    direction: TcAttachType,
) -> anyhow::Result<()> {
    let dir = dir_label(direction);

    // TCX (kernel >= 6.6): insert at head so we run before all other TC programs.
    match prog.attach_with_options(
        iface,
        direction,
        TcAttachOptions::TcxOrder(LinkOrder::first()),
    ) {
        Ok(_) => {
            info!("attached {dir} via TCX (head of chain) to {iface}");
            return Ok(());
        }
        Err(e) => {
            info!("TCX not available for {dir}, falling back to legacy TC: {e}");
        }
    }

    // Legacy TC: use the lowest usable priority (1) so Retina runs as early as
    // possible. If another program already holds priority 1, ordering within
    // the same priority depends on insertion order — whoever attached first
    // runs first, and Retina may miss packets dropped by the earlier program.
    prog.attach_with_options(
        iface,
        direction,
        TcAttachOptions::Netlink(NlOptions {
            priority: 1,
            handle: 0,
        }),
    )?;
    info!("attached {dir} via legacy TC (priority 1) to {iface}");
    Ok(())
}

/// Attach a TC classifier to an endpoint veth, returning the owned link for
/// lifetime-based detach. Same TCX-first strategy as [`attach_tc`].
fn attach_tc_linked(
    prog: &mut SchedClassifier,
    iface: &str,
    direction: TcAttachType,
) -> anyhow::Result<SchedClassifierLink> {
    let dir = dir_label(direction);

    // TCX (kernel >= 6.6): insert at head so we run before all other TC programs.
    match prog.attach_with_options(
        iface,
        direction,
        TcAttachOptions::TcxOrder(LinkOrder::first()),
    ) {
        Ok(id) => {
            info!("attached {dir} via TCX (head of chain) to {iface}");
            return prog.take_link(id).context("take_link after TCX attach");
        }
        Err(e) => {
            info!("TCX not available for {dir}, falling back to legacy TC: {e}");
        }
    }

    let id = prog.attach_with_options(
        iface,
        direction,
        TcAttachOptions::Netlink(NlOptions {
            priority: 1,
            handle: 0,
        }),
    )?;
    info!("attached {dir} via legacy TC (priority 1) to {iface}");
    prog.take_link(id)
        .context("take_link after legacy TC attach")
}

fn dir_label(direction: TcAttachType) -> &'static str {
    match direction {
        TcAttachType::Ingress => "ingress",
        TcAttachType::Egress => "egress",
        TcAttachType::Custom(_) => "custom",
    }
}

/// Load eBPF programs and optionally attach host classifiers.
///
/// When `extra_interfaces` is non-empty, `host_ingress`/`host_egress` are
/// loaded and attached to each listed interface. When empty, host programs
/// are skipped entirely.
///
/// Endpoint programs (`endpoint_ingress`/`endpoint_egress`) are always loaded
/// but NOT attached — use [`attach_endpoint`] to attach them to individual
/// veth interfaces.
///
/// `sampling_rate`: 0 or 1 = no sampling, N = report ~1/N packets.
/// `ring_buffer_size`: size in bytes for the BPF ring buffer (must be power of 2).
pub(crate) fn load_and_attach(
    extra_interfaces: &[String],
    sampling_rate: u32,
    ring_buffer_size: u32,
) -> anyhow::Result<EbpfHandles> {
    let use_ringbuf = kernel_supports_ringbuf();
    info!(
        use_ringbuf,
        "selecting event buffer type based on kernel version"
    );

    let mut ebpf = if use_ringbuf {
        info!(ring_buffer_size, "loading ringbuf eBPF variant");
        EbpfLoader::new()
            .map_max_entries("EVENTS", ring_buffer_size)
            .load(&EBPF_OBJ_RINGBUF.bytes)
            .context("failed to load ringbuf eBPF variant")?
    } else {
        Ebpf::load(&EBPF_OBJ_PERF.bytes).context("failed to load perf eBPF variant")?
    };

    if !extra_interfaces.is_empty() {
        let prog: &mut SchedClassifier = ebpf
            .program_mut("host_ingress")
            .context("eBPF program 'host_ingress' not found")?
            .try_into()?;
        prog.load()
            .context("failed to verify host_ingress eBPF program")?;

        let prog: &mut SchedClassifier = ebpf
            .program_mut("host_egress")
            .context("eBPF program 'host_egress' not found")?
            .try_into()?;
        prog.load()
            .context("failed to verify host_egress eBPF program")?;

        for iface in extra_interfaces {
            let _ = aya::programs::tc::qdisc_add_clsact(iface);

            let prog: &mut SchedClassifier = ebpf
                .program_mut("host_ingress")
                .context("eBPF program 'host_ingress' not found")?
                .try_into()?;
            attach_tc(prog, iface, TcAttachType::Ingress)?;

            let prog: &mut SchedClassifier = ebpf
                .program_mut("host_egress")
                .context("eBPF program 'host_egress' not found")?
                .try_into()?;
            attach_tc(prog, iface, TcAttachType::Egress)?;
        }
    }

    // Load endpoint programs (verify bytecode) but don't attach yet.
    let prog: &mut SchedClassifier = ebpf
        .program_mut("endpoint_ingress")
        .context("eBPF program 'endpoint_ingress' not found")?
        .try_into()?;
    prog.load()
        .context("failed to verify endpoint_ingress eBPF program")?;
    info!("loaded endpoint_ingress");

    let prog: &mut SchedClassifier = ebpf
        .program_mut("endpoint_egress")
        .context("eBPF program 'endpoint_egress' not found")?
        .try_into()?;
    prog.load()
        .context("failed to verify endpoint_egress eBPF program")?;
    info!("loaded endpoint_egress");

    // Set sampling rate in RETINA_CONFIG map (index 0).
    {
        let mut config_map = Array::<_, u32>::try_from(
            ebpf.map_mut("RETINA_CONFIG")
                .context("eBPF map 'RETINA_CONFIG' not found")?,
        )?;
        config_map
            .set(0, sampling_rate, 0)
            .context("failed to set sampling rate in RETINA_CONFIG")?;
        info!(sampling_rate, "set RETINA_CONFIG[0]");
    }

    // Extract maps (take ownership for 'static lifetime).
    let event_source = if use_ringbuf {
        EventSource::Ring(RingBuf::try_from(
            ebpf.take_map("EVENTS")
                .context("eBPF map 'EVENTS' not found")?,
        )?)
    } else {
        EventSource::Perf(PerfEventArray::try_from(
            ebpf.take_map("EVENTS")
                .context("eBPF map 'EVENTS' not found")?,
        )?)
    };

    let conntrack = HashMap::try_from(
        ebpf.take_map("CONNTRACK")
            .context("eBPF map 'CONNTRACK' not found")?,
    )?;

    Ok((ebpf, event_source, conntrack))
}

/// Attach `endpoint_ingress` + `endpoint_egress` TC classifiers to a veth.
///
/// Returns owned [`SchedClassifierLink`]s — dropping them auto-detaches the
/// programs from the interface.
pub(crate) fn attach_endpoint(
    ebpf: &mut Ebpf,
    iface: &str,
) -> anyhow::Result<(SchedClassifierLink, SchedClassifierLink)> {
    // Add clsact qdisc (ignore error if already exists).
    let _ = aya::programs::tc::qdisc_add_clsact(iface);

    let prog: &mut SchedClassifier = ebpf
        .program_mut("endpoint_ingress")
        .context("eBPF program 'endpoint_ingress' not found")?
        .try_into()?;
    let ingress_link = attach_tc_linked(prog, iface, TcAttachType::Ingress)?;

    let prog: &mut SchedClassifier = ebpf
        .program_mut("endpoint_egress")
        .context("eBPF program 'endpoint_egress' not found")?
        .try_into()?;
    let egress_link = attach_tc_linked(prog, iface, TcAttachType::Egress)?;

    Ok((ingress_link, egress_link))
}
