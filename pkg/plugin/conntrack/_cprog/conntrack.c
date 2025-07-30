//go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "compiler.h"
#include "bpf_helpers.h"
#include "conntrack.h"
#include "dynamic.h"

struct tcpmetadata {
	__u32 seq; // TCP sequence number
	__u32 ack_num; // TCP ack number
	__u32 tsval; // TCP timestamp value
	__u32 tsecr; // TCP timestamp echo reply
};

struct conntrackmetadata {
    /*
        bytes_*_count indicates the number of bytes sent and received in the forward and reply direction.
        These values will be based on the conntrack entry.
    */
    __u64 bytes_tx_count;
    __u64 bytes_rx_count;
    /*
        packets_*_count indicates the number of packets sent and received in the forward and reply direction.
        These values will be based on the conntrack entry.
    */
    __u32 packets_tx_count;
    __u32 packets_rx_count;
};

/**
 *  The structure representing the count of observed TCP flags.
 *  To observe new flags, they should be added to this structure and _ct_record_tcp_flags updated.
 */
struct tcpflagscount
{
    __u32 syn;
    __u32 ack;
    __u32 fin;
    __u32 rst;
    __u32 psh;
    __u32 urg;
    __u32 ece;
    __u32 cwr;
    __u32 ns;
};

struct packet
{
    __u64 t_nsec; // timestamp in nanoseconds
    __u32 bytes; // packet size in bytes
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    struct tcpmetadata tcp_metadata; // TCP metadata
    __u8 observation_point;
    __u8 traffic_direction;
    __u8 proto;
    __u16 flags; // For TCP packets, this is the TCP flags. For UDP packets, this is will always be 1 for conntrack purposes.
    bool is_reply;
    __u32 previously_observed_packets; // When sampling, this is the number of observed packets since the last report.
    __u32 previously_observed_bytes; // When sampling, this is the number of observed bytes since the last report.
    struct tcpflagscount previously_observed_flags; // When sampling, this is the previously observed TCP flags since the last report.
    struct conntrackmetadata conntrack_metadata;
};

/**
 * The structure representing whether or not to report a packet and associated metadata.
 */
struct packetreport
{
    __u32 previously_observed_packets;
    __u32 previously_observed_bytes;
    struct tcpflagscount previously_observed_flags;
    bool report;
};

/**
 * The structure representing an ipv4 5-tuple key in the connection tracking map.
 */
struct ct_v4_key {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8 proto;
};

/**
 * The structure representing a connection in the connection tracking map.
 */
struct ct_entry {
    __u32 eviction_time; // eviction_time stores the time when the connection should be evicted from the map.
    /**
     * last_report_*_dir stores the time when the last packet event was reported in the send and reply direction respectively.
     */
    __u32 last_report_tx_dir;
    __u32 last_report_rx_dir;
    /**
     * bytes_seen_since_last_report_*_dir stores the number of bytes observed since the last packet event was reported.
     */
    __u32 bytes_seen_since_last_report_tx_dir;
    __u32 bytes_seen_since_last_report_rx_dir;
    /**
     * packets_seen_since_last_report_*_dir stores the number of packets observed since the last packet event was reported.
     */
    __u32 packets_seen_since_last_report_tx_dir;
    __u32 packets_seen_since_last_report_rx_dir;
    /**
     * flags_seen_since_last_report_*_dir stores the number of TCP flags observed since the last packet event was reported.
     */
    struct tcpflagscount flags_seen_since_last_report_tx_dir;
    struct tcpflagscount flags_seen_since_last_report_rx_dir;
    /**
     * traffic_direction indicates the direction of the connection in relation to the host. 
     * If the connection is initiated from within the host, the traffic_direction is egress. Otherwise, the traffic_direction is ingress.
     */
    __u8 traffic_direction;
    /**
     * flags_seen_*_dir stores the flags seen in the send and reply direction respectively.
     */
    __u8  flags_seen_tx_dir;
    __u8  flags_seen_rx_dir;
    /**
     * is_direction_unknown is set to true if the direction of the connection is unknown. This can happen if the connection is created
     * before retina deployment and the SYN packet was not captured.
     */
    bool is_direction_unknown;
    struct conntrackmetadata conntrack_metadata;
};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, struct ct_v4_key);
    __type(value, struct ct_entry);
    __uint(max_entries, CT_MAP_SIZE);
    __uint(pinning, LIBBPF_PIN_BY_NAME); // needs pinning so this can be access from other processes .i.e debug cli
} retina_conntrack SEC(".maps");

/**
 * Helper function to update the count of observed TCP flags.
 * @arg flags The observed flags.
 * @arg count The TCP flag count to update.
 */
static inline void _ct_record_tcp_flags(__u8 flags, struct tcpflagscount *count) {
    if (!count) {
        return;
    }
    if (flags & TCP_SYN) {
        if (count->syn < UINT32_MAX) {
           count->syn += 1;
        }
    }
    if (flags & TCP_ACK) {
        if (count->ack < UINT32_MAX) {
            count->ack += 1;
        }
    }
    if (flags & TCP_FIN) {
        if (count->fin < UINT32_MAX) {
            count->fin += 1;
        }
    }
    if (flags & TCP_RST) {
        if (count->rst < UINT32_MAX) {
            count->rst += 1;
        }
    }
    if (flags & TCP_PSH) {
        if (count->psh < UINT32_MAX) {
            count->psh += 1;
        }
    }
    if (flags & TCP_URG) {
        if (count->urg < UINT32_MAX) {
            count->urg += 1;
        }
    }
    if (flags & TCP_ECE) {
        if (count->ece < UINT32_MAX) {
            count->ece += 1;
        }
    }
    if (flags & TCP_CWR) {
        if (count->cwr < UINT32_MAX) {
            count->cwr += 1;
        }
    }
    if (flags & TCP_NS) {
        if (count->ns < UINT32_MAX) {
            count->ns += 1;
        }
    }
}

/**
 * Helper function to reverse a key.
 * @arg reverse_key The key to store the reversed key.
 * @arg key The key to be reversed.
 */
static inline void _ct_reverse_key(struct ct_v4_key *reverse_key, const struct ct_v4_key *key) {
    if (!reverse_key || !key) {
        return;
    }
    reverse_key->src_ip = key->dst_ip;
    reverse_key->dst_ip = key->src_ip;
    reverse_key->src_port = key->dst_port;
    reverse_key->dst_port = key->src_port;
    reverse_key->proto = key->proto;
}

/**
 * Returns the traffic direction based on the observation point.
 * @arg observation_point The point in the network stack where the packet is observed.
 */
static __always_inline __u8 _ct_get_traffic_direction(__u8 observation_point) {
    if (observation_point == OBSERVATION_POINT_FROM_ENDPOINT || observation_point == OBSERVATION_POINT_TO_NETWORK) {
        return TRAFFIC_DIRECTION_EGRESS;
    } else if (observation_point == OBSERVATION_POINT_TO_ENDPOINT || observation_point == OBSERVATION_POINT_FROM_NETWORK) {
        return TRAFFIC_DIRECTION_INGRESS;
    } else {
        return TRAFFIC_DIRECTION_UNKNOWN;
    }
}

/**
 * Create a new TCP connection.
 * @arg *p pointer to the packet to be processed.
 * @arg key The key to be used to create the new connection.
 * @arg observation_point The point in the network stack where the packet is observed.
 * @arg is_reply true if the packet is a SYN-ACK packet. False if it is a SYN packet.
 */
static __always_inline bool _ct_create_new_tcp_connection(struct packet *p, struct ct_v4_key key,  __u8 observation_point, bool is_reply) {
    struct ct_entry new_value;
    __builtin_memset(&new_value, 0, sizeof(struct ct_entry));
    __u64 now = bpf_mono_now();
    // Check for overflow
    if (CT_SYN_TIMEOUT > UINT32_MAX - now) {
        return false;
    }
    new_value.eviction_time = now + CT_SYN_TIMEOUT;
    if(is_reply) {
        new_value.flags_seen_rx_dir = p->flags;
        new_value.last_report_rx_dir = now;
        new_value.bytes_seen_since_last_report_rx_dir = 0;
        new_value.packets_seen_since_last_report_rx_dir = 0;
    } else {
        new_value.flags_seen_tx_dir = p->flags;
        new_value.last_report_tx_dir = now;
        new_value.bytes_seen_since_last_report_tx_dir = 0;
        new_value.packets_seen_since_last_report_tx_dir = 0;
    }
    new_value.is_direction_unknown = false;
    new_value.traffic_direction = _ct_get_traffic_direction(observation_point);

    #ifdef ENABLE_CONNTRACK_METRICS
        if(is_reply){
            new_value.conntrack_metadata.packets_rx_count = 1;
            new_value.conntrack_metadata.bytes_rx_count = p->bytes;
        } else {
            new_value.conntrack_metadata.packets_tx_count = 1;
            new_value.conntrack_metadata.bytes_tx_count = p->bytes;
        }
        // Update initial conntrack metadata for the connection.
        __builtin_memcpy(&p->conntrack_metadata, &new_value.conntrack_metadata, sizeof(struct conntrackmetadata));
    #endif // ENABLE_CONNTRACK_METRICS

    // Update packet
    p->is_reply = is_reply;
    p->traffic_direction = new_value.traffic_direction;
    bpf_map_update_elem(&retina_conntrack, &key, &new_value, BPF_ANY);
    return true;
}

/**
 * Create a new UDP connection.
 * @arg *p pointer to the packet to be processed.
 * @arg key The key to be used to create the new connection.
 * @arg observation_point The point in the network stack where the packet is observed.
 */
static __always_inline bool _ct_handle_udp_connection(struct packet *p, struct ct_v4_key key, __u8 observation_point) {
    struct ct_entry new_value;
    __builtin_memset(&new_value, 0, sizeof(struct ct_entry));
    __u64 now = bpf_mono_now();
    // Check for overflow
    if (CT_CONNECTION_LIFETIME_NONTCP > UINT32_MAX - now) {
        return false;
    }
    new_value.eviction_time = now + CT_CONNECTION_LIFETIME_NONTCP;
    new_value.flags_seen_tx_dir = p->flags;
    new_value.last_report_tx_dir = now;
    new_value.bytes_seen_since_last_report_tx_dir = 0;
    new_value.packets_seen_since_last_report_tx_dir = 0;
    new_value.traffic_direction = _ct_get_traffic_direction(observation_point);
    #ifdef ENABLE_CONNTRACK_METRICS
        new_value.conntrack_metadata.packets_tx_count = 1;
        new_value.conntrack_metadata.bytes_tx_count = p->bytes;
        // Update packet's conntrack metadata.
        __builtin_memcpy(&p->conntrack_metadata, &new_value.conntrack_metadata, sizeof(struct conntrackmetadata));;
    #endif // ENABLE_CONNTRACK_METRICS    

    // Update packet
    p->is_reply = false;
    p->traffic_direction = new_value.traffic_direction;
    bpf_map_update_elem(&retina_conntrack, &key, &new_value, BPF_ANY);
    return true;
}

/**
 * Handle a TCP connection.
 * @arg *p pointer to the packet to be processed.
 * @arg key The key to be used to handle the connection.
 * @arg reverse_key The reverse key to be used to handle the connection.
 * @arg observation_point The point in the network stack where the packet is observed.
 */
static __always_inline bool _ct_handle_tcp_connection(struct packet *p, struct ct_v4_key key, struct ct_v4_key reverse_key, __u8 observation_point) {
    u8 tcp_handshake = p->flags & (TCP_SYN|TCP_ACK);
    if (tcp_handshake == TCP_SYN) {
        // We have a SYN, we set `is_reply` to false and we provide `key`
        return _ct_create_new_tcp_connection(p, key, observation_point, false);
    } else if(tcp_handshake == TCP_SYN|TCP_ACK) {
        // We have a SYN-ACK, we set `is_reply` to true and we provide `reverse_key`
        return _ct_create_new_tcp_connection(p, reverse_key, observation_point, true);
    }

    // The packet is not a SYN packet and the connection corresponding to this packet is not found.
    // This might be because of an ongoing connection that started before Retina started tracking connections.
    // Therefore we would have missed the SYN packet. A conntrack entry will be created with best effort.
    struct ct_entry new_value;
    __builtin_memset(&new_value, 0, sizeof(struct ct_entry));
    __u64 now = bpf_mono_now();
    // Check for overflow
    if (CT_CONNECTION_LIFETIME_TCP > UINT32_MAX - now) {
        return false;
    }
    // Set the connection as unknown direction since we did not capture the SYN packet.
    new_value.is_direction_unknown = true;
    new_value.eviction_time = now + CT_CONNECTION_LIFETIME_TCP;
    new_value.traffic_direction = _ct_get_traffic_direction(observation_point);
    p->traffic_direction = new_value.traffic_direction;

    // Check for ACK flag. If the ACK flag is set, the packet is considered as a packet in the reply direction of the connection.
    if (p->flags & TCP_ACK) {
        p->is_reply = true;
        new_value.flags_seen_rx_dir = p->flags;
        new_value.last_report_rx_dir = now;
        new_value.bytes_seen_since_last_report_rx_dir = 0;
        new_value.packets_seen_since_last_report_rx_dir = 0;
        #ifdef ENABLE_CONNTRACK_METRICS
            new_value.conntrack_metadata.bytes_rx_count = p->bytes;
            new_value.conntrack_metadata.packets_rx_count = 1;
        #endif // ENABLE_CONNTRACK_METRICS
        bpf_map_update_elem(&retina_conntrack, &reverse_key, &new_value, BPF_ANY);
    } else { // Otherwise, the packet is considered as a packet in the send direction.
        p->is_reply = false;
        new_value.flags_seen_tx_dir = p->flags;
        new_value.last_report_tx_dir = now;
        new_value.bytes_seen_since_last_report_tx_dir = 0;
        new_value.packets_seen_since_last_report_tx_dir = 0;
        #ifdef ENABLE_CONNTRACK_METRICS
            new_value.conntrack_metadata.bytes_tx_count = p->bytes;
            new_value.conntrack_metadata.packets_tx_count = 1;
        #endif // ENABLE_CONNTRACK_METRICS
        bpf_map_update_elem(&retina_conntrack, &key, &new_value, BPF_ANY);
    }
    #ifdef ENABLE_CONNTRACK_METRICS
        // Update packet's conntrack metadata.
        __builtin_memcpy(&p->conntrack_metadata, &new_value.conntrack_metadata, sizeof(struct conntrackmetadata));
    #endif // ENABLE_CONNTRACK_METRICS
    return true;
}

/**
 * Handle a new connection.
 * @arg *p pointer to the packet to be processed.
 * @arg key The key to be used to handle the connection.
 * @arg reverse_key The reverse key to be used to handle the connection.
 * @arg observation_point The point in the network stack where the packet is observed.
 */
static __always_inline struct packetreport _ct_handle_new_connection(struct packet *p, struct ct_v4_key key, struct ct_v4_key reverse_key, __u8 observation_point) {
    struct packetreport report;
    __builtin_memset(&report, 0, sizeof(struct packetreport));
    if (key.proto & IPPROTO_TCP) {
        report.report = _ct_handle_tcp_connection(p, key, reverse_key, observation_point);
    } else if (key.proto & IPPROTO_UDP) {
        report.report = _ct_handle_udp_connection(p, key, observation_point);
    } else {
        report.report = false; // We are not interested in other protocols.
    }
    return report;
}

/**
 * Check if a packet should be reported to userspace. Update the corresponding conntrack entry.
 * @arg key The key of the connection in Retina's conntrack map.
 * @arg entry The entry of the connection in Retina's conntrack map.
 * @arg flags The flags of the packet.
 * @arg direction The direction of the packet in relation to the connection.
 * @arg bytes The size of the packet in bytes.
 * Returns a packetreport struct representing if the packet should be reported to userspace.
 */
static __always_inline struct packetreport _ct_should_report_packet(struct ct_v4_key *key, struct ct_entry *entry, __u8 flags, __u8 direction, __u32 bytes) {
    struct packetreport report;
    __builtin_memset(&report, 0, sizeof(struct packetreport));
    report.report = false;

    // Check for null parameters.
    if (!entry || !key) {
        return report;
    }
    
    // Get direction-specific data
    __u8 seen_flags;
    __u32 last_report;
    __u32 packets_seen;
    __u32 bytes_seen;
    if (direction == CT_PACKET_DIR_TX) {
        seen_flags = READ_ONCE(entry->flags_seen_tx_dir);
        last_report = READ_ONCE(entry->last_report_tx_dir);
        bytes_seen = READ_ONCE(entry->bytes_seen_since_last_report_tx_dir);
        packets_seen = READ_ONCE(entry->packets_seen_since_last_report_tx_dir);
        __builtin_memcpy(&report.previously_observed_flags, &entry->flags_seen_since_last_report_tx_dir, sizeof(struct tcpflagscount));
    } else {
        seen_flags = READ_ONCE(entry->flags_seen_rx_dir);
        last_report = READ_ONCE(entry->last_report_rx_dir);
        bytes_seen = READ_ONCE(entry->bytes_seen_since_last_report_rx_dir);
        packets_seen = READ_ONCE(entry->packets_seen_since_last_report_rx_dir);
        __builtin_memcpy(&report.previously_observed_flags, &entry->flags_seen_since_last_report_rx_dir, sizeof(struct tcpflagscount));
    }

    report.previously_observed_bytes = bytes_seen;
    report.previously_observed_packets = packets_seen;

    // Check for overflow
    if (bytes_seen <= UINT32_MAX-bytes) {
        bytes_seen += bytes;
    }

    if (packets_seen <= UINT32_MAX-1) {
        packets_seen += 1;
    }

    __u64 now = bpf_mono_now();
    __u32 eviction_time = READ_ONCE(entry->eviction_time);

    // Check if the connection timed out
    if (now >= eviction_time) {
        bpf_map_delete_elem(&retina_conntrack, key);
        report.report = true;
        return report; // Report the last packet received before deletion
    }

    __u8 packet_flags = flags;

    // OR the seen flags with the new flags
    flags |= seen_flags;
    __u8 protocol = key->proto;

    // Handle connection state updates and reporting conditions
    bool should_report = false;

    // TCP-specific state management
    if (protocol == IPPROTO_TCP) {
        // Handle TCP connection termination states
        
        // Check if this is the final ACK in TCP connection teardown
        // (Both directions have seen FIN, and this is just an ACK without other control flags)
        if ((flags & TCP_ACK) && 
            !(flags & (TCP_FIN | TCP_SYN | TCP_RST)) && 
            (entry->flags_seen_tx_dir & TCP_FIN) && 
            (entry->flags_seen_rx_dir & TCP_FIN)) {
            bpf_map_delete_elem(&retina_conntrack, key);
            report.report = true;
            return report; // Report final ACK before connection removal
        }

        // If RST is seen, delete connection immediately
        if (flags & TCP_RST) {
            bpf_map_delete_elem(&retina_conntrack, key);
            report.report = true;
            return report; // Report RST before connection removal
        }

        // Update FIN flag status in the appropriate direction
        if (packet_flags & TCP_FIN) {
            if (direction == CT_PACKET_DIR_TX) {
                entry->flags_seen_tx_dir |= TCP_FIN;
            } else {
                entry->flags_seen_rx_dir |= TCP_FIN;
            }
            should_report = true; // Always report FIN packets
        }

        // Always report important TCP control flags
        if (packet_flags & (TCP_SYN | TCP_URG | TCP_ECE | TCP_CWR)) {
            should_report = true;
        }

        // If FIN seen in both directions, transition to TIME_WAIT state
        if ((entry->flags_seen_tx_dir & TCP_FIN) && (entry->flags_seen_rx_dir & TCP_FIN)) {
            WRITE_ONCE(entry->eviction_time, now + CT_TIME_WAIT_TIMEOUT_TCP);
            should_report = true; // Report transition to TIME_WAIT
        } else {
            // Extend TCP connection lifetime
            WRITE_ONCE(entry->eviction_time, now + CT_CONNECTION_LIFETIME_TCP);
        }
    } else if (protocol == IPPROTO_UDP) {
        // Extend UDP/other connection lifetime
        WRITE_ONCE(entry->eviction_time, now + CT_CONNECTION_LIFETIME_NONTCP);
    }

    // Report if:
    // 1. We already decided to report based on protocol-specific rules, or
    // 2. New flags have appeared, or
    // 3. Reporting interval has elapsed
    if (should_report || flags != seen_flags || now - last_report >= CT_REPORT_INTERVAL) {
        report.report = true;
        // Update the connection's state
        if (direction == CT_PACKET_DIR_TX) {
            WRITE_ONCE(entry->flags_seen_tx_dir, flags);
            WRITE_ONCE(entry->last_report_tx_dir, now);
            WRITE_ONCE(entry->bytes_seen_since_last_report_tx_dir, 0);
            WRITE_ONCE(entry->packets_seen_since_last_report_tx_dir, 0);
            __builtin_memset(&entry->flags_seen_since_last_report_tx_dir, 0, sizeof(struct tcpflagscount));
        } else {
            WRITE_ONCE(entry->flags_seen_rx_dir, flags);
            WRITE_ONCE(entry->last_report_rx_dir, now);
            WRITE_ONCE(entry->bytes_seen_since_last_report_rx_dir, 0);
            WRITE_ONCE(entry->packets_seen_since_last_report_rx_dir, 0);
            __builtin_memset(&entry->flags_seen_since_last_report_rx_dir, 0, sizeof(struct tcpflagscount));
        }
        return report;
    } else {
        struct tcpflagscount newcount;
        __builtin_memcpy(&newcount, &report.previously_observed_flags, sizeof(struct tcpflagscount));
        _ct_record_tcp_flags(packet_flags, &newcount);
        if (direction == CT_PACKET_DIR_TX) {
            WRITE_ONCE(entry->bytes_seen_since_last_report_tx_dir, bytes_seen);
            WRITE_ONCE(entry->packets_seen_since_last_report_tx_dir, packets_seen);
            __builtin_memcpy(&entry->flags_seen_since_last_report_tx_dir, &newcount, sizeof(struct tcpflagscount));
        } else {
            WRITE_ONCE(entry->bytes_seen_since_last_report_rx_dir, bytes_seen);
            WRITE_ONCE(entry->packets_seen_since_last_report_rx_dir, packets_seen);
            __builtin_memcpy(&entry->flags_seen_since_last_report_rx_dir, &newcount, sizeof(struct tcpflagscount));
        }
    }

    return report;
}

/**
 * Process a packet and update the connection tracking map.
 * @arg *p pointer to the packet to be processed.
 * @arg observation_point The point in the network stack where the packet is observed.
 * Returns a packetreport struct representing if the packet should be reported to userspace.
 */
static __always_inline __attribute__((unused)) struct packetreport ct_process_packet(struct packet *p, __u8 observation_point) {
    if (!p) {
        struct packetreport report;
        __builtin_memset(&report, 0, sizeof(struct packetreport));
        report.report = false;
        report.previously_observed_packets = 0;
        report.previously_observed_bytes = 0;
        return report;
    }

    // Create a new key for the send direction.
    struct ct_v4_key key;
    __builtin_memset(&key, 0, sizeof(struct ct_v4_key));
    key.src_ip = p->src_ip;
    key.dst_ip = p->dst_ip;
    key.src_port = p->src_port;
    key.dst_port = p->dst_port;
    key.proto = p->proto;

    // Lookup the connection in the map.
    struct ct_entry *entry = bpf_map_lookup_elem(&retina_conntrack, &key);

    // If the connection is found in the send direction, update the connection.
    if (entry) {
        // Update the packet accordingly.
        p->is_reply = false;
        p->traffic_direction = entry->traffic_direction;
        #ifdef ENABLE_CONNTRACK_METRICS
            // Update packet count and bytes count on conntrack entry.
            WRITE_ONCE(entry->conntrack_metadata.packets_tx_count, READ_ONCE(entry->conntrack_metadata.packets_tx_count) + 1);
            WRITE_ONCE(entry->conntrack_metadata.bytes_tx_count, READ_ONCE(entry->conntrack_metadata.bytes_tx_count) + p->bytes);
            // Update packet's conntract metadata.
            __builtin_memcpy(&p->conntrack_metadata, &entry->conntrack_metadata, sizeof(struct conntrackmetadata));
        #endif // ENABLE_CONNTRACK_METRICS
        return _ct_should_report_packet(&key, entry, p->flags, CT_PACKET_DIR_TX, p->bytes);
    }
    
    // The connection is not found in the send direction. Check the reply direction by reversing the key.
    struct ct_v4_key reverse_key;
    __builtin_memset(&reverse_key, 0, sizeof(struct ct_v4_key));
    _ct_reverse_key(&reverse_key, &key);
    // Lookup the connection in the map based on the reverse key.
    entry = bpf_map_lookup_elem(&retina_conntrack, &reverse_key);

    // If the connection is found based on the reverse key, meaning that the packet is a reply packet to an existing connection.
    if (entry) {
        // Update the packet accordingly.
        p->is_reply = true;
        p->traffic_direction = entry->traffic_direction;
        #ifdef ENABLE_CONNTRACK_METRICS
            // Update packet count and bytes count on conntrack entry.
            WRITE_ONCE(entry->conntrack_metadata.packets_rx_count, READ_ONCE(entry->conntrack_metadata.packets_rx_count) + 1);
            WRITE_ONCE(entry->conntrack_metadata.bytes_rx_count, READ_ONCE(entry->conntrack_metadata.bytes_rx_count) + p->bytes);
            // Update packet's conntract metadata.
            __builtin_memcpy(&p->conntrack_metadata, &entry->conntrack_metadata, sizeof(struct conntrackmetadata));
        #endif // ENABLE_CONNTRACK_METRICS
        return _ct_should_report_packet(&reverse_key, entry, p->flags, CT_PACKET_DIR_RX, p->bytes);
    }

    // If the connection is still not found, the connection is new.
    return _ct_handle_new_connection(p, key, reverse_key, observation_point);
}