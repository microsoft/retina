#![no_std]

// ── Direction constants (shared with retina-common for consistency) ──────────

pub const DIR_UNKNOWN: u8 = 0;
pub const DIR_INGRESS: u8 = 1;
pub const DIR_EGRESS: u8 = 2;

// ── Netfilter hook constants ────────────────────────────────────────────────

/// Hooks <= NF_INET_FORWARD are ingress, >= NF_INET_LOCAL_OUT are egress.
pub const NF_INET_PRE_ROUTING: u32 = 0;
pub const NF_INET_LOCAL_IN: u32 = 1;
pub const NF_INET_FORWARD: u32 = 2;
pub const NF_INET_LOCAL_OUT: u32 = 3;
pub const NF_INET_POST_ROUTING: u32 = 4;

pub const NF_DROP: u32 = 0;

// ── Linux errno used for filtering ───────────────────────────────────────────

/// `EAGAIN` (11): non-blocking accept with empty queue returns `-EAGAIN`.
pub const EAGAIN: i32 = 11;

// ── sockaddr_in field offsets (stable UAPI, never changes) ──────────────────

/// `sockaddr_in->sin_port` (`__be16`). Offset 2 bytes.
pub const SOCKADDR_IN_PORT: usize = 2;
/// `sockaddr_in->sin_addr` (`struct in_addr`, i.e. `__be32`). Offset 4 bytes.
pub const SOCKADDR_IN_ADDR: usize = 4;

// ── DROPREASON_OFFSETS array map indices ─────────────────────────────────────
//
// Shared between eBPF (reader) and userspace loader (writer).
// The loader resolves these from kernel BTF at startup and writes them
// into the `DROPREASON_OFFSETS` BPF array map.

pub const OFFSET_SKB_LEN: u32 = 0;
pub const OFFSET_SKB_TRANSPORT_HEADER: u32 = 1;
pub const OFFSET_SKB_NETWORK_HEADER: u32 = 2;
pub const OFFSET_SKB_HEAD: u32 = 3;
pub const OFFSET_SOCK_SKC_DADDR: u32 = 4;
pub const OFFSET_SOCK_SKC_RCV_SADDR: u32 = 5;
pub const OFFSET_SOCK_SKC_DPORT: u32 = 6;
pub const OFFSET_SOCK_SKC_NUM: u32 = 7;
pub const OFFSET_NF_HOOK_STATE_HOOK: u32 = 8;

/// Total number of slots in the offsets map (room to grow).
pub const OFFSET_MAP_SIZE: u32 = 16;

// ── Drop reason codes ───────────────────────────────────────────────────────

#[repr(u8)]
#[derive(Copy, Clone, Default, PartialEq, Eq)]
pub enum DropReason {
    #[default]
    IptableRuleDrop = 0,
    IptableNatDrop = 1,
    TcpConnectDrop = 2,
    TcpAcceptDrop = 3,
    ConntrackDrop = 4,
    Unknown = 5,
}

impl DropReason {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::IptableRuleDrop => "IPTABLE_RULE_DROP",
            Self::IptableNatDrop => "IPTABLE_NAT_DROP",
            Self::TcpConnectDrop => "TCP_CONNECT_DROP",
            Self::TcpAcceptDrop => "TCP_ACCEPT_DROP",
            Self::ConntrackDrop => "CONNTRACK_DROP",
            Self::Unknown => "UNKNOWN",
        }
    }

    pub fn from_u8(v: u8) -> Self {
        match v {
            0 => Self::IptableRuleDrop,
            1 => Self::IptableNatDrop,
            2 => Self::TcpConnectDrop,
            3 => Self::TcpAcceptDrop,
            4 => Self::ConntrackDrop,
            _ => Self::Unknown,
        }
    }
}

// ── Event struct emitted from eBPF to userspace ─────────────────────────────

/// 32 bytes, power-of-2 aligned. Emitted via RingBuf or PerfEventArray.
#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct DropEvent {
    /// Timestamp from `bpf_ktime_get_boot_ns()`.
    pub ts_ns: u64,
    /// Source IPv4 address (host byte order).
    pub src_ip: u32,
    /// Destination IPv4 address (host byte order).
    pub dst_ip: u32,
    /// Source port (host byte order).
    pub src_port: u16,
    /// Destination port (host byte order).
    pub dst_port: u16,
    /// Packet length from `skb->len`, or 0 for sock-based hooks.
    pub bytes: u32,
    /// IP protocol number (6 = TCP, 17 = UDP).
    pub proto: u8,
    /// [`DropReason`] discriminant.
    pub drop_reason: u8,
    /// Traffic direction: [`DIR_UNKNOWN`], [`DIR_INGRESS`], or [`DIR_EGRESS`].
    pub direction: u8,
    /// Kernel return value (truncated to i8; errno range -125..0 fits).
    pub return_val: i8,
    /// PID (tgid) of the process that triggered the drop, or 0 if unavailable.
    pub pid: u32,
}

// ── Metrics map types ───────────────────────────────────────────────────────

/// Key for the per-CPU metrics hash map.
#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct DropMetricsKey {
    pub drop_reason: u8,
    pub direction: u8,
    pub _pad: [u8; 2],
    pub return_val: i32,
}

/// Value for the per-CPU metrics hash map.
#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct DropMetricsValue {
    pub count: u64,
    pub bytes: u64,
}

// ── aya::Pod impls (userspace only) ─────────────────────────────────────────

#[cfg(feature = "userspace")]
unsafe impl aya::Pod for DropEvent {}
#[cfg(feature = "userspace")]
unsafe impl aya::Pod for DropMetricsKey {}
#[cfg(feature = "userspace")]
unsafe impl aya::Pod for DropMetricsValue {}

// ── Helper: convert nf_hook number to direction ─────────────────────────────

/// Map a netfilter hook number to a traffic direction.
#[inline]
pub fn nf_hook_to_direction(hook: u32) -> u8 {
    match hook {
        NF_INET_PRE_ROUTING | NF_INET_LOCAL_IN | NF_INET_FORWARD => DIR_INGRESS,
        NF_INET_LOCAL_OUT | NF_INET_POST_ROUTING => DIR_EGRESS,
        _ => DIR_UNKNOWN,
    }
}
