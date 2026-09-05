//! `no_std` C-compatible structs shared between eBPF and userspace for the
//! packetparser plugin: `PacketEvent`, conntrack types, and protocol constants.
#![no_std]

// ── Observation points ───────────────────────────────────────────────
pub const OBS_FROM_ENDPOINT: u8 = 0;
pub const OBS_TO_ENDPOINT: u8 = 1;
pub const OBS_FROM_NETWORK: u8 = 2;
pub const OBS_TO_NETWORK: u8 = 3;

// ── Traffic direction ────────────────────────────────────────────────
pub const DIR_UNKNOWN: u8 = 0;
pub const DIR_INGRESS: u8 = 1;
pub const DIR_EGRESS: u8 = 2;

// ── TCP flags ────────────────────────────────────────────────────────
pub const TCP_FIN: u16 = 0x01;
pub const TCP_SYN: u16 = 0x02;
pub const TCP_RST: u16 = 0x04;
pub const TCP_PSH: u16 = 0x08;
pub const TCP_ACK: u16 = 0x10;
pub const TCP_URG: u16 = 0x20;
pub const TCP_ECE: u16 = 0x40;
pub const TCP_CWR: u16 = 0x80;
pub const TCP_NS: u16 = 0x100;

// ── Conntrack timeouts (seconds) ─────────────────────────────────────
pub const CT_LIFETIME_TCP: u32 = 360;
pub const CT_TIME_WAIT_TCP: u32 = 30;
pub const CT_LIFETIME_NONTCP: u32 = 60;
pub const CT_SYN_TIMEOUT: u32 = 60;
pub const CT_REPORT_INTERVAL: u32 = 30;
pub const CT_MAP_SIZE: u32 = 262_144;

// ── IP protocol numbers ──────────────────────────────────────────────
pub const IPPROTO_TCP: u8 = 6;
pub const IPPROTO_UDP: u8 = 17;

// ── Shared types ─────────────────────────────────────────────────────

#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct TcpFlagsCount {
    pub syn: u32,
    pub ack: u32,
    pub fin: u32,
    pub rst: u32,
    pub psh: u32,
    pub urg: u32,
    pub ece: u32,
    pub cwr: u32,
    pub ns: u32,
}

#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct ConntrackMetadata {
    pub bytes_tx: u64,
    pub bytes_rx: u64,
    pub pkts_tx: u32,
    pub pkts_rx: u32,
}

#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct PacketEvent {
    pub ts_ns: u64,
    pub bytes: u32,
    pub src_ip: u32,
    pub dst_ip: u32,
    pub src_port: u16,
    pub dst_port: u16,
    pub proto: u8,
    pub observation_point: u8,
    pub traffic_direction: u8,
    pub _pad1: u8,
    pub flags: u16,
    pub is_reply: u8,
    pub _pad2: u8,
    pub previously_observed_packets: u32,
    pub previously_observed_bytes: u32,
    pub prev_flags: TcpFlagsCount,
    pub _pad3: u32,
    pub ct_metadata: ConntrackMetadata,
}

#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct CtV4Key {
    pub src_ip: u32,
    pub dst_ip: u32,
    pub src_port: u16,
    pub dst_port: u16,
    pub proto: u8,
    pub _pad1: u8,
    pub _pad2: u16,
}

#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct CtEntry {
    pub eviction_time: u32,
    pub last_report_tx_dir: u32,
    pub last_report_rx_dir: u32,
    pub bytes_since_report_tx: u32,
    pub bytes_since_report_rx: u32,
    pub pkts_since_report_tx: u32,
    pub pkts_since_report_rx: u32,
    pub flags_since_report_tx: TcpFlagsCount,
    pub flags_since_report_rx: TcpFlagsCount,
    pub traffic_direction: u8,
    pub flags_seen_tx: u8,
    pub flags_seen_rx: u8,
    pub is_direction_unknown: u8,
    pub ct_metadata: ConntrackMetadata,
}

// Safety: these are plain C-compatible structs with no pointers
#[cfg(feature = "userspace")]
unsafe impl aya::Pod for PacketEvent {}
#[cfg(feature = "userspace")]
unsafe impl aya::Pod for CtV4Key {}
#[cfg(feature = "userspace")]
unsafe impl aya::Pod for CtEntry {}
#[cfg(feature = "userspace")]
unsafe impl aya::Pod for TcpFlagsCount {}
#[cfg(feature = "userspace")]
unsafe impl aya::Pod for ConntrackMetadata {}
