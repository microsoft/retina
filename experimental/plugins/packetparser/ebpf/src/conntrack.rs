use aya_ebpf::maps::LruHashMap;
use retina_common::*;

/// Direction constants for conntrack processing.
const CT_PACKET_DIR_TX: u8 = 0x00;
const CT_PACKET_DIR_RX: u8 = 0x01;

/// Helper: get monotonic time in seconds.
#[inline(always)]
fn bpf_mono_now() -> u32 {
    (unsafe { aya_ebpf::helpers::bpf_ktime_get_boot_ns() } / 1_000_000_000) as u32
}

/// Determine traffic direction from observation point.
#[inline(always)]
fn ct_get_traffic_direction(obs_point: u8) -> u8 {
    match obs_point {
        OBS_FROM_ENDPOINT | OBS_TO_NETWORK => DIR_EGRESS,
        OBS_TO_ENDPOINT | OBS_FROM_NETWORK => DIR_INGRESS,
        _ => DIR_UNKNOWN,
    }
}

/// Build a reverse key by swapping src/dst.
#[inline(always)]
fn ct_reverse_key(key: &CtV4Key) -> CtV4Key {
    CtV4Key {
        src_ip: key.dst_ip,
        dst_ip: key.src_ip,
        src_port: key.dst_port,
        dst_port: key.src_port,
        proto: key.proto,
        _pad1: 0,
        _pad2: 0,
    }
}

/// Record TCP flags into a TcpFlagsCount struct.
#[inline(always)]
fn ct_record_tcp_flags(flags: u16, count: &mut TcpFlagsCount) {
    if flags & TCP_SYN != 0 {
        count.syn = count.syn.saturating_add(1);
    }
    if flags & TCP_ACK != 0 {
        count.ack = count.ack.saturating_add(1);
    }
    if flags & TCP_FIN != 0 {
        count.fin = count.fin.saturating_add(1);
    }
    if flags & TCP_RST != 0 {
        count.rst = count.rst.saturating_add(1);
    }
    if flags & TCP_PSH != 0 {
        count.psh = count.psh.saturating_add(1);
    }
    if flags & TCP_URG != 0 {
        count.urg = count.urg.saturating_add(1);
    }
    if flags & TCP_ECE != 0 {
        count.ece = count.ece.saturating_add(1);
    }
    if flags & TCP_CWR != 0 {
        count.cwr = count.cwr.saturating_add(1);
    }
    if flags & TCP_NS != 0 {
        count.ns = count.ns.saturating_add(1);
    }
}

/// Result of conntrack processing.
pub struct PacketReport {
    pub report: bool,
    pub previously_observed_packets: u32,
    pub previously_observed_bytes: u32,
    pub previously_observed_flags: TcpFlagsCount,
}

impl PacketReport {
    #[inline(always)]
    fn empty() -> Self {
        Self {
            report: false,
            previously_observed_packets: 0,
            previously_observed_bytes: 0,
            previously_observed_flags: TcpFlagsCount::default(),
        }
    }
}

/// Check whether an existing connection should be reported.
/// Updates the conntrack entry in-place.
#[inline(always)]
fn ct_should_report_packet(
    conntrack: &LruHashMap<CtV4Key, CtEntry>,
    key: &CtV4Key,
    entry: &mut CtEntry,
    flags: u8,
    direction: u8,
    bytes: u32,
    sampled: bool,
) -> PacketReport {
    let mut report = PacketReport::empty();

    // Snapshot direction-specific data.
    let (seen_flags, last_report, bytes_seen, packets_seen, prev_flags) =
        if direction == CT_PACKET_DIR_TX {
            (
                entry.flags_seen_tx,
                entry.last_report_tx_dir,
                entry.bytes_since_report_tx,
                entry.pkts_since_report_tx,
                entry.flags_since_report_tx,
            )
        } else {
            (
                entry.flags_seen_rx,
                entry.last_report_rx_dir,
                entry.bytes_since_report_rx,
                entry.pkts_since_report_rx,
                entry.flags_since_report_rx,
            )
        };

    report.previously_observed_bytes = bytes_seen;
    report.previously_observed_packets = packets_seen;
    report.previously_observed_flags = prev_flags;

    let new_bytes = bytes_seen.saturating_add(bytes);
    let new_packets = packets_seen.saturating_add(1);

    let now = bpf_mono_now();
    let eviction_time = entry.eviction_time;

    // Connection timed out.
    if now >= eviction_time {
        let _ = conntrack.remove(key);
        report.report = true;
        return report;
    }

    let packet_flags = flags;
    let combined_flags = flags | seen_flags;
    let protocol = key.proto;

    let mut should_report = false;

    if protocol == IPPROTO_TCP {
        // Final ACK after both FINs seen.
        if (combined_flags & TCP_ACK as u8) != 0
            && (combined_flags & (TCP_FIN as u8 | TCP_SYN as u8 | TCP_RST as u8)) == 0
            && (entry.flags_seen_tx & TCP_FIN as u8) != 0
            && (entry.flags_seen_rx & TCP_FIN as u8) != 0
        {
            let _ = conntrack.remove(key);
            report.report = true;
            return report;
        }

        // RST: delete immediately.
        if (combined_flags & TCP_RST as u8) != 0 {
            let _ = conntrack.remove(key);
            report.report = true;
            return report;
        }

        // FIN in this packet.
        if (packet_flags & TCP_FIN as u8) != 0 {
            if direction == CT_PACKET_DIR_TX {
                entry.flags_seen_tx |= TCP_FIN as u8;
            } else {
                entry.flags_seen_rx |= TCP_FIN as u8;
            }
            should_report = true;
        }

        // Important control flags.
        if (packet_flags & (TCP_SYN as u8 | TCP_URG as u8 | TCP_ECE as u8 | TCP_CWR as u8)) != 0
        {
            should_report = true;
        }

        // Both FINs seen: transition to TIME_WAIT.
        if (entry.flags_seen_tx & TCP_FIN as u8) != 0
            && (entry.flags_seen_rx & TCP_FIN as u8) != 0
        {
            entry.eviction_time = now.saturating_add(CT_TIME_WAIT_TCP);
            should_report = true;
        } else {
            entry.eviction_time = now.saturating_add(CT_LIFETIME_TCP);
        }
    } else if protocol == IPPROTO_UDP {
        entry.eviction_time = now.saturating_add(CT_LIFETIME_NONTCP);
    }

    // Update combined flags.
    if combined_flags != seen_flags {
        if direction == CT_PACKET_DIR_TX {
            entry.flags_seen_tx = combined_flags;
        } else {
            entry.flags_seen_rx = combined_flags;
        }
    }

    // Decide whether to report.
    // Control flags and periodic interval always trigger.
    // New flag combinations only trigger when the packet is sampled.
    if should_report
        || (sampled && combined_flags != seen_flags)
        || now.wrapping_sub(last_report) >= CT_REPORT_INTERVAL
    {
        report.report = true;
        // Reset counters on report.
        if direction == CT_PACKET_DIR_TX {
            entry.last_report_tx_dir = now;
            entry.bytes_since_report_tx = 0;
            entry.pkts_since_report_tx = 0;
            entry.flags_since_report_tx = TcpFlagsCount::default();
        } else {
            entry.last_report_rx_dir = now;
            entry.bytes_since_report_rx = 0;
            entry.pkts_since_report_rx = 0;
            entry.flags_since_report_rx = TcpFlagsCount::default();
        }
    } else {
        // Accumulate counters.
        let mut new_flag_count = prev_flags;
        ct_record_tcp_flags(packet_flags as u16, &mut new_flag_count);
        if direction == CT_PACKET_DIR_TX {
            entry.bytes_since_report_tx = new_bytes;
            entry.pkts_since_report_tx = new_packets;
            entry.flags_since_report_tx = new_flag_count;
        } else {
            entry.bytes_since_report_rx = new_bytes;
            entry.pkts_since_report_rx = new_packets;
            entry.flags_since_report_rx = new_flag_count;
        }
    }

    report
}

/// Create a new TCP connection entry.
#[inline(always)]
fn ct_create_new_tcp_connection(
    conntrack: &LruHashMap<CtV4Key, CtEntry>,
    pkt: &mut PacketEvent,
    key: &CtV4Key,
    obs_point: u8,
    is_reply: bool,
    sampled: bool,
) -> PacketReport {
    let now = bpf_mono_now();
    let timeout = if (pkt.flags & TCP_SYN) != 0 && (pkt.flags & TCP_ACK) == 0 {
        CT_SYN_TIMEOUT
    } else {
        CT_LIFETIME_TCP
    };

    let eviction_time = match now.checked_add(timeout) {
        Some(t) => t,
        None => return PacketReport::empty(),
    };

    let mut entry = CtEntry::default();
    entry.eviction_time = eviction_time;
    entry.is_direction_unknown = 0;
    entry.traffic_direction = ct_get_traffic_direction(obs_point);

    if is_reply {
        entry.flags_seen_rx = pkt.flags as u8;
        entry.last_report_rx_dir = if sampled { now } else { 0 };
        entry.ct_metadata.pkts_rx = 1;
        entry.ct_metadata.bytes_rx = pkt.bytes as u64;
        if !sampled {
            entry.bytes_since_report_rx = pkt.bytes;
            entry.pkts_since_report_rx = 1;
            ct_record_tcp_flags(pkt.flags, &mut entry.flags_since_report_rx);
        }
    } else {
        entry.flags_seen_tx = pkt.flags as u8;
        entry.last_report_tx_dir = if sampled { now } else { 0 };
        entry.ct_metadata.pkts_tx = 1;
        entry.ct_metadata.bytes_tx = pkt.bytes as u64;
        if !sampled {
            entry.bytes_since_report_tx = pkt.bytes;
            entry.pkts_since_report_tx = 1;
            ct_record_tcp_flags(pkt.flags, &mut entry.flags_since_report_tx);
        }
    }

    pkt.is_reply = is_reply as u8;
    pkt.traffic_direction = entry.traffic_direction;
    pkt.ct_metadata = entry.ct_metadata;

    let _ = conntrack.insert(key, &entry, 0);

    PacketReport {
        report: sampled,
        previously_observed_packets: 0,
        previously_observed_bytes: 0,
        previously_observed_flags: TcpFlagsCount::default(),
    }
}

/// Handle a new TCP connection (SYN / SYN-ACK / mid-stream).
#[inline(always)]
fn ct_handle_tcp_connection(
    conntrack: &LruHashMap<CtV4Key, CtEntry>,
    pkt: &mut PacketEvent,
    key: &CtV4Key,
    reverse_key: &CtV4Key,
    obs_point: u8,
    sampled: bool,
) -> PacketReport {
    let handshake = pkt.flags & (TCP_SYN | TCP_ACK);

    if handshake == TCP_SYN {
        return ct_create_new_tcp_connection(conntrack, pkt, key, obs_point, false, sampled);
    }
    if handshake == (TCP_SYN | TCP_ACK) {
        return ct_create_new_tcp_connection(conntrack, pkt, reverse_key, obs_point, true, sampled);
    }

    // Mid-stream: missed the handshake.
    let now = bpf_mono_now();
    let eviction_time = match now.checked_add(CT_LIFETIME_TCP) {
        Some(t) => t,
        None => return PacketReport::empty(),
    };

    let mut entry = CtEntry::default();
    entry.eviction_time = eviction_time;
    entry.is_direction_unknown = 1;
    entry.traffic_direction = ct_get_traffic_direction(obs_point);
    pkt.traffic_direction = entry.traffic_direction;

    // If ACK is set, treat as reply direction.
    if (pkt.flags & TCP_ACK) != 0 {
        pkt.is_reply = 1;
        entry.flags_seen_rx = pkt.flags as u8;
        entry.last_report_rx_dir = if sampled { now } else { 0 };
        entry.ct_metadata.bytes_rx = pkt.bytes as u64;
        entry.ct_metadata.pkts_rx = 1;
        if !sampled {
            entry.bytes_since_report_rx = pkt.bytes;
            entry.pkts_since_report_rx = 1;
            ct_record_tcp_flags(pkt.flags, &mut entry.flags_since_report_rx);
        }
        pkt.ct_metadata = entry.ct_metadata;
        let _ = conntrack.insert(reverse_key, &entry, 0);
    } else {
        pkt.is_reply = 0;
        entry.flags_seen_tx = pkt.flags as u8;
        entry.last_report_tx_dir = if sampled { now } else { 0 };
        entry.ct_metadata.bytes_tx = pkt.bytes as u64;
        entry.ct_metadata.pkts_tx = 1;
        if !sampled {
            entry.bytes_since_report_tx = pkt.bytes;
            entry.pkts_since_report_tx = 1;
            ct_record_tcp_flags(pkt.flags, &mut entry.flags_since_report_tx);
        }
        pkt.ct_metadata = entry.ct_metadata;
        let _ = conntrack.insert(key, &entry, 0);
    }

    PacketReport {
        report: sampled,
        previously_observed_packets: 0,
        previously_observed_bytes: 0,
        previously_observed_flags: TcpFlagsCount::default(),
    }
}

/// Handle a new UDP connection.
#[inline(always)]
fn ct_handle_udp_connection(
    conntrack: &LruHashMap<CtV4Key, CtEntry>,
    pkt: &mut PacketEvent,
    key: &CtV4Key,
    obs_point: u8,
    sampled: bool,
) -> PacketReport {
    let now = bpf_mono_now();
    let eviction_time = match now.checked_add(CT_LIFETIME_NONTCP) {
        Some(t) => t,
        None => return PacketReport::empty(),
    };

    let mut entry = CtEntry::default();
    entry.eviction_time = eviction_time;
    entry.traffic_direction = ct_get_traffic_direction(obs_point);
    entry.flags_seen_tx = pkt.flags as u8;
    entry.last_report_tx_dir = if sampled { now } else { 0 };
    entry.ct_metadata.pkts_tx = 1;
    entry.ct_metadata.bytes_tx = pkt.bytes as u64;

    if !sampled {
        entry.bytes_since_report_tx = pkt.bytes;
        entry.pkts_since_report_tx = 1;
    }

    pkt.is_reply = 0;
    pkt.traffic_direction = entry.traffic_direction;
    pkt.ct_metadata = entry.ct_metadata;

    let _ = conntrack.insert(key, &entry, 0);

    PacketReport {
        report: sampled,
        previously_observed_packets: 0,
        previously_observed_bytes: 0,
        previously_observed_flags: TcpFlagsCount::default(),
    }
}

/// Main conntrack entry point: process a packet and decide whether to report.
#[inline(always)]
pub fn ct_process_packet(
    conntrack: &LruHashMap<CtV4Key, CtEntry>,
    pkt: &mut PacketEvent,
    obs_point: u8,
    sampled: bool,
) -> PacketReport {
    // Build forward key.
    let key = CtV4Key {
        src_ip: pkt.src_ip,
        dst_ip: pkt.dst_ip,
        src_port: pkt.src_port,
        dst_port: pkt.dst_port,
        proto: pkt.proto,
        _pad1: 0,
        _pad2: 0,
    };

    // Lookup forward direction.
    if let Some(entry) = conntrack.get_ptr_mut(&key) {
        let entry = unsafe { &mut *entry };
        pkt.is_reply = 0;
        pkt.traffic_direction = entry.traffic_direction;

        // Update conntrack metadata.
        entry.ct_metadata.pkts_tx = entry.ct_metadata.pkts_tx.saturating_add(1);
        entry.ct_metadata.bytes_tx = entry.ct_metadata.bytes_tx.saturating_add(pkt.bytes as u64);
        pkt.ct_metadata = entry.ct_metadata;

        return ct_should_report_packet(conntrack, &key, entry, pkt.flags as u8, CT_PACKET_DIR_TX, pkt.bytes, sampled);
    }

    // Lookup reverse direction.
    let reverse_key = ct_reverse_key(&key);
    if let Some(entry) = conntrack.get_ptr_mut(&reverse_key) {
        let entry = unsafe { &mut *entry };
        pkt.is_reply = 1;
        pkt.traffic_direction = entry.traffic_direction;

        // Update conntrack metadata.
        entry.ct_metadata.pkts_rx = entry.ct_metadata.pkts_rx.saturating_add(1);
        entry.ct_metadata.bytes_rx = entry.ct_metadata.bytes_rx.saturating_add(pkt.bytes as u64);
        pkt.ct_metadata = entry.ct_metadata;

        return ct_should_report_packet(conntrack, &reverse_key, entry, pkt.flags as u8, CT_PACKET_DIR_RX, pkt.bytes, sampled);
    }

    // New connection.
    if key.proto == IPPROTO_TCP {
        ct_handle_tcp_connection(conntrack, pkt, &key, &reverse_key, obs_point, sampled)
    } else if key.proto == IPPROTO_UDP {
        ct_handle_udp_connection(conntrack, pkt, &key, obs_point, sampled)
    } else {
        PacketReport::empty()
    }
}
