use aya::{
    maps::{HashMap, MapData, PerfEventArray},
    programs::{
        tc::{SchedClassifierLink, TcAttachType},
        SchedClassifier,
    },
    Ebpf,
};
use retina_common::{CtEntry, CtV4Key};
use tracing::info;

pub type EbpfHandles = (Ebpf, PerfEventArray<MapData>, HashMap<MapData, CtV4Key, CtEntry>);

/// Force 8-byte alignment on embedded byte data so the `object` crate's ELF
/// parser can cast the pointer to `Elf64_Ehdr` without misalignment.
/// (`include_bytes!` only guarantees 1-byte alignment.)
#[repr(C, align(8))]
struct Align8<Bytes: ?Sized> {
    bytes: Bytes,
}

/// eBPF object embedded at compile time with guaranteed 8-byte alignment.
/// Requires `cargo xtask build-ebpf --release` to run first.
static EBPF_OBJ: &Align8<[u8]> = &Align8 {
    bytes: *include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../ebpf/target/bpfel-unknown-none/release/packetparser-ebpf"
    )),
};

/// Load eBPF programs and optionally attach host classifiers.
///
/// When `host_iface` is `Some`, `host_ingress`/`host_egress` are loaded and
/// attached to that interface. When `None`, host programs are skipped entirely.
///
/// Endpoint programs (`endpoint_ingress`/`endpoint_egress`) are always loaded
/// but NOT attached — use [`attach_endpoint`] to attach them to individual
/// veth interfaces.
pub fn load_and_attach(host_iface: Option<&str>) -> anyhow::Result<EbpfHandles> {
    let mut ebpf = Ebpf::load(&EBPF_OBJ.bytes)?;

    if let Some(iface) = host_iface {
        let _ = aya::programs::tc::qdisc_add_clsact(iface);

        let prog: &mut SchedClassifier =
            ebpf.program_mut("host_ingress").unwrap().try_into()?;
        prog.load()?;
        prog.attach(iface, TcAttachType::Ingress)?;
        info!("attached host_ingress to {iface}");

        let prog: &mut SchedClassifier =
            ebpf.program_mut("host_egress").unwrap().try_into()?;
        prog.load()?;
        prog.attach(iface, TcAttachType::Egress)?;
        info!("attached host_egress to {iface}");
    }

    // Load endpoint programs (verify bytecode) but don't attach yet.
    let prog: &mut SchedClassifier = ebpf.program_mut("endpoint_ingress").unwrap().try_into()?;
    prog.load()?;
    info!("loaded endpoint_ingress");

    let prog: &mut SchedClassifier = ebpf.program_mut("endpoint_egress").unwrap().try_into()?;
    prog.load()?;
    info!("loaded endpoint_egress");

    // Extract maps (take ownership for 'static lifetime).
    let perf_array = PerfEventArray::try_from(ebpf.take_map("EVENTS").unwrap())?;
    let conntrack = HashMap::try_from(ebpf.take_map("CONNTRACK").unwrap())?;

    Ok((ebpf, perf_array, conntrack))
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

    let prog: &mut SchedClassifier = ebpf.program_mut("endpoint_ingress").unwrap().try_into()?;
    let ingress_id = prog.attach(iface, TcAttachType::Ingress)?;
    let ingress_link = prog.take_link(ingress_id)?;

    let prog: &mut SchedClassifier = ebpf.program_mut("endpoint_egress").unwrap().try_into()?;
    let egress_id = prog.attach(iface, TcAttachType::Egress)?;
    let egress_link = prog.take_link(egress_id)?;

    Ok((ingress_link, egress_link))
}
