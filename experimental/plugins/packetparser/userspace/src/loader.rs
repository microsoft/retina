use anyhow::Context as _;
use aya::{
    Ebpf, EbpfLoader,
    maps::{Array, HashMap, MapData, PerfEventArray, RingBuf},
    programs::{
        SchedClassifier,
        tc::{SchedClassifierLink, TcAttachType},
    },
};
use retina_common::{CtEntry, CtV4Key};
use tracing::info;

/// Event source abstraction: ring buffer (Linux 5.8+) or perf event array.
pub enum EventSource {
    Perf(PerfEventArray<MapData>),
    Ring(RingBuf<MapData>),
}

pub type EbpfHandles = (Ebpf, EventSource, HashMap<MapData, CtV4Key, CtEntry>);

/// Force 8-byte alignment on embedded byte data so the `object` crate's ELF
/// parser can cast the pointer to `Elf64_Ehdr` without misalignment.
/// (`include_bytes!` only guarantees 1-byte alignment.)
#[repr(C, align(8))]
struct Align8<Bytes: ?Sized> {
    bytes: Bytes,
}

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

/// Check if the running kernel supports BPF ring buffers (>= 5.8).
fn kernel_supports_ringbuf() -> bool {
    unsafe {
        let mut utsname: libc::utsname = core::mem::zeroed();
        if libc::uname(&mut utsname) != 0 {
            return false;
        }
        let release = core::ffi::CStr::from_ptr(utsname.release.as_ptr());
        let release = release.to_string_lossy();
        // Parse "major.minor..." from uname release string.
        let mut parts = release.split('.');
        let major: u32 = parts.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        let minor: u32 = parts.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        major > 5 || (major == 5 && minor >= 8)
    }
}

/// Load eBPF programs and optionally attach host classifiers.
///
/// When `host_iface` is `Some`, `host_ingress`/`host_egress` are loaded and
/// attached to that interface. When `None`, host programs are skipped entirely.
///
/// Endpoint programs (`endpoint_ingress`/`endpoint_egress`) are always loaded
/// but NOT attached — use [`attach_endpoint`] to attach them to individual
/// veth interfaces.
///
/// `sampling_rate`: 0 or 1 = no sampling, N = report ~1/N packets.
/// `ring_buffer_size`: size in bytes for the BPF ring buffer (must be power of 2).
pub fn load_and_attach(
    host_iface: Option<&str>,
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
            .load(&EBPF_OBJ_RINGBUF.bytes)?
    } else {
        Ebpf::load(&EBPF_OBJ_PERF.bytes)?
    };

    if let Some(iface) = host_iface {
        let _ = aya::programs::tc::qdisc_add_clsact(iface);

        let prog: &mut SchedClassifier = ebpf
            .program_mut("host_ingress")
            .context("eBPF program 'host_ingress' not found")?
            .try_into()?;
        prog.load()?;
        prog.attach(iface, TcAttachType::Ingress)?;
        info!("attached host_ingress to {iface}");

        let prog: &mut SchedClassifier = ebpf
            .program_mut("host_egress")
            .context("eBPF program 'host_egress' not found")?
            .try_into()?;
        prog.load()?;
        prog.attach(iface, TcAttachType::Egress)?;
        info!("attached host_egress to {iface}");
    }

    // Load endpoint programs (verify bytecode) but don't attach yet.
    let prog: &mut SchedClassifier = ebpf
        .program_mut("endpoint_ingress")
        .context("eBPF program 'endpoint_ingress' not found")?
        .try_into()?;
    prog.load()?;
    info!("loaded endpoint_ingress");

    let prog: &mut SchedClassifier = ebpf
        .program_mut("endpoint_egress")
        .context("eBPF program 'endpoint_egress' not found")?
        .try_into()?;
    prog.load()?;
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
pub fn attach_endpoint(
    ebpf: &mut Ebpf,
    iface: &str,
) -> anyhow::Result<(SchedClassifierLink, SchedClassifierLink)> {
    // Add clsact qdisc (ignore error if already exists).
    let _ = aya::programs::tc::qdisc_add_clsact(iface);

    let prog: &mut SchedClassifier = ebpf
        .program_mut("endpoint_ingress")
        .context("eBPF program 'endpoint_ingress' not found")?
        .try_into()?;
    let ingress_id = prog.attach(iface, TcAttachType::Ingress)?;
    let ingress_link = prog.take_link(ingress_id)?;

    let prog: &mut SchedClassifier = ebpf
        .program_mut("endpoint_egress")
        .context("eBPF program 'endpoint_egress' not found")?
        .try_into()?;
    let egress_id = prog.attach(iface, TcAttachType::Egress)?;
    let egress_link = prog.take_link(egress_id)?;

    Ok((ingress_link, egress_link))
}
