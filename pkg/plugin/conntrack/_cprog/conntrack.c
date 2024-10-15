// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "compiler.h"
#include "bpf_helpers.h"
#include "conntrack.h"

struct tcpmetadata {
	__u32 seq; // TCP sequence number
	__u32 ack_num; // TCP ack number
	__u32 tsval; // TCP timestamp value
	__u32 tsecr; // TCP timestamp echo reply
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
	__u8 flags; // For TCP packets, this is the TCP flags. For UDP packets, this is will always be 1 for conntrack purposes.
	bool is_reply;
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
     * last_seq stores the last sequence number seen in the connection.
     */
    __u32 last_seq;
};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, struct ct_v4_key);
    __type(value, struct ct_entry);
    __uint(max_entries, CT_MAP_SIZE);
    __uint(pinning, LIBBPF_PIN_BY_NAME); // needs pinning so this can be access from other processes .i.e debug cli
} retina_conntrack SEC(".maps");

/**
 * Helper function to check if a packet sequence number is increasing.
 */
static inline bool is_seq_increasing(__u32 seq1, __u32 seq2) {
    return (__s32)(seq1 - seq2) > 0;
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
 * @arg key The key to be used to create the new connection.
 * @arg flags The flags of the packet.
 * @arg observation_point The point in the network stack where the packet is observed.
 */
static __always_inline bool _ct_create_new_tcp_connection(struct ct_v4_key key, __u8 flags, __u8 observation_point) {
    struct ct_entry new_value;
    __builtin_memset(&new_value, 0, sizeof(struct ct_entry));
    __u64 now = bpf_mono_now();
    // Check for overflow
    if (CT_SYN_TIMEOUT > UINT32_MAX - now) {
        return false;
    }
    new_value.eviction_time = now + CT_SYN_TIMEOUT;
    new_value.flags_seen_tx_dir = flags;
    new_value.traffic_direction = _ct_get_traffic_direction(observation_point);
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
    new_value.traffic_direction = _ct_get_traffic_direction(observation_point);
    bpf_map_update_elem(&retina_conntrack, &key, &new_value, BPF_ANY);
    // Update packet
    p->is_reply = false;
    p->traffic_direction = new_value.traffic_direction;
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
    struct ct_entry *entry = bpf_map_lookup_elem(&retina_conntrack, &key);
    struct ct_entry *reverse_entry = bpf_map_lookup_elem(&retina_conntrack, &reverse_key);
    if (entry && reverse_entry) {
        // Compare sequence numbers to determine direction
        if (is_seq_increasing(p->tcp_metadata.seq, entry->last_seq)) {
            p->is_reply = false;
            entry->last_seq = p->tcp_metadata.seq;
            bpf_map_update_elem(&retina_conntrack, &key, entry, BPF_ANY);
        } else {
            p->is_reply = true;
            reverse_entry->last_seq = p->tcp_metadata.seq;
            bpf_map_update_elem(&retina_conntrack, &reverse_key, reverse_entry, BPF_ANY);
        }
    } else {
        // Check if the packet is a SYN packet.
        if (p->flags & TCP_SYN) {
            // Update packet accordingly.
            p->is_reply = false;
            p->traffic_direction = _ct_get_traffic_direction(observation_point);
            // Create a new connection with a timeout of CT_SYN_TIMEOUT.
            return _ct_create_new_tcp_connection(key, p->flags, observation_point);
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
        new_value.eviction_time = now + CT_CONNECTION_LIFETIME_TCP;
        new_value.traffic_direction = _ct_get_traffic_direction(observation_point);
        p->traffic_direction = new_value.traffic_direction;

        // Check for ACK flag. If the ACK flag is set, the packet is considered as a packet in the reply direction of the connection.
        if (p->flags & TCP_ACK) {
            p->is_reply = true;
            new_value.flags_seen_rx_dir = p->flags;
            new_value.last_report_rx_dir = now;
            bpf_map_update_elem(&retina_conntrack, &reverse_key, &new_value, BPF_ANY);
        } else { // Otherwise, the packet is considered as a packet in the send direction.
            p->is_reply = false;
            new_value.flags_seen_tx_dir = p->flags;
            new_value.last_report_tx_dir = now;
            bpf_map_update_elem(&retina_conntrack, &key, &new_value, BPF_ANY);
        }
    }
    return true;
}

/**
 * Handle a new connection.
 * @arg *p pointer to the packet to be processed.
 * @arg key The key to be used to handle the connection.
 * @arg reverse_key The reverse key to be used to handle the connection.
 * @arg observation_point The point in the network stack where the packet is observed.
 */
static __always_inline bool _ct_handle_new_connection(struct packet *p, struct ct_v4_key key, struct ct_v4_key reverse_key, __u8 observation_point) {
    if (key.proto & IPPROTO_TCP) {
        return _ct_handle_tcp_connection(p, key, reverse_key, observation_point);
    } else if (key.proto & IPPROTO_UDP) {
        return _ct_handle_udp_connection(p, key, observation_point);
    } else {
        return false; // We are not interested in other protocols.
    }
}

/**
 * Check if a packet should be reported to userspace. Update the corresponding conntrack entry.
 * @arg flags The flags of the packet.
 * @arg entry The entry of the connection in Retina's conntrack map.
 * @arg direction The direction of the packet in relation to the connection.
 * @arg protocol The protocol of the packet (TCP or UDP).
 * Returns true if the packet should be reported to userspace. False otherwise.
 */
static __always_inline bool _ct_should_report_packet(struct ct_entry *entry, __u8 flags, __u8 direction, struct ct_v4_key *key) {
    // Check for null parameters.
    if (!entry) {
        return false;
    }

    __u8 protocol = key->proto;
    __u64 now = bpf_mono_now();
    __u32 eviction_time = READ_ONCE(entry->eviction_time);
    __u8 seen_flags;
    __u32 last_report;
    if (direction == CT_PACKET_DIR_TX) {
        seen_flags = READ_ONCE(entry->flags_seen_tx_dir);
        last_report = READ_ONCE(entry->last_report_tx_dir);
    } else {
        seen_flags = READ_ONCE(entry->flags_seen_rx_dir);
        last_report = READ_ONCE(entry->last_report_rx_dir);
    }
    // OR the seen flags with the new flags.
    flags |= seen_flags;

    // Check if the connection timed out or if it is a TCP connection and FIN or RST flags are set.
    if (now >= eviction_time || (protocol == IPPROTO_TCP && flags & (TCP_FIN | TCP_RST))) {
        // The connection is closing or closed. Delete the connection from the map
        bpf_map_delete_elem(&retina_conntrack, key);

        return true; // Report the last packet received.
    }
    // Update the eviction time of the connection.
    if (protocol == IPPROTO_TCP) {
        // Check for overflow, only update the eviction time if there is no overflow.
        if (CT_CONNECTION_LIFETIME_TCP > UINT32_MAX - now) {
            return false;
        }
        WRITE_ONCE(entry->eviction_time, now + CT_CONNECTION_LIFETIME_TCP);
    } else {
        if (CT_CONNECTION_LIFETIME_NONTCP > UINT32_MAX - now) {
            return false;
        }
        WRITE_ONCE(entry->eviction_time, now + CT_CONNECTION_LIFETIME_NONTCP);
    }
    // We will only report this packet iff a new flag is seen for the given direction or the report interval has passed.
    if (flags != seen_flags || now - last_report >= CT_REPORT_INTERVAL) {
        if (direction == CT_PACKET_DIR_TX) {
            WRITE_ONCE(entry->flags_seen_tx_dir, flags);
            WRITE_ONCE(entry->last_report_tx_dir, now);
        } else {
            WRITE_ONCE(entry->flags_seen_rx_dir, flags);
            WRITE_ONCE(entry->last_report_rx_dir, now);
        }
        return true;
    }
    return false;
}

/**
 * Process a packet and update the connection tracking map.
 * @arg *p pointer to the packet to be processed.
 * @arg observation_point The point in the network stack where the packet is observed.
 * Returns true if the packet should be report to userspace. False otherwise.
 */
static __always_inline __attribute__((unused)) bool ct_process_packet(struct packet *p, __u8 observation_point) {

    if (!p) {
        return false;
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
        return _ct_should_report_packet(entry, p->flags, CT_PACKET_DIR_TX, &key);
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
        return _ct_should_report_packet(entry, p->flags, CT_PACKET_DIR_RX, &key);
    }

    // If the connection is still not found, the connection is new.
    return _ct_handle_new_connection(p, key, reverse_key, observation_point);
}
