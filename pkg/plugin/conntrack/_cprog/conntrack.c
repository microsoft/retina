// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "compiler.h"
#include "bpf_helpers.h"
#include "conntrack.h"


/**
    * The structure representing an ipv4 5-tuple key in the connection tracking map.
 **/
struct ct_v4_key {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8 proto;
};
/**
    * The structure representing a connection in the connection tracking map.
 **/
struct ct_entry {
    __u32 lifetime; // lifetime stores the time when the connection should be removed from the map.
    /*
        * traffic_direction indicates the direction of the connection in relation to the host. 
        * If the connection is initiated from within the host, the traffic_direction is egress. Otherwise, the traffic_direction is ingress.
    */
    enum ct_traffic_dir traffic_direction;
    /*
        * flags_seen_*_dir stores the flags seen in the forward and reply direction.
    */
    __u8  flags_seen_forward_dir;
    __u8  flags_seen_reply_dir;
    /*
        * last_report_*_dir stores the time when the last packet event was reported in the forward and reply direction respectively.
    */
    __u32 last_report_forward_dir;
    __u32 last_report_reply_dir;
    __u8 is_closing; // is_closing indicates if the connection is closing.
};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, struct ct_v4_key);
    __type(value, struct ct_entry);
    __uint(max_entries, CT_MAP_SIZE);
} retina_conntrack_map SEC(".maps");

/*
    * Returns the traffic direction based on the observation point.
    * @arg observation_point The point in the network stack where the packet is observed.
*/
static __always_inline __u8 _ct_get_traffic_direction(enum obs_point observation_point) {
    switch (observation_point) {
        case FROM_ENDPOINT:
            return TRAFFIC_DIRECTION_EGRESS;
        case TO_ENDPOINT:
            return TRAFFIC_DIRECTION_INGRESS;
        default:
            return TRAFFIC_DIRECTION_UNKNOWN;
    }
}

/*
    * Create a new TCP connection.
    * @arg key The key to be used to create the new connection.
    * @arg flags The flags of the packet.
    * @arg observation_point The point in the network stack where the packet is observed.
    * @arg timeout The timeout for the connection.
*/
static __always_inline bool _ct_create_new_tcp_connection(struct ct_v4_key key, __u8 flags, enum obs_point observation_point, __u64 timeout) {
    struct ct_entry new_value;
    __builtin_memset(&new_value, 0, sizeof(struct ct_entry));
    __u64 now = bpf_mono_now();
    new_value.lifetime = now + timeout;
    new_value.flags_seen_forward_dir = flags;
    new_value.traffic_direction = _ct_get_traffic_direction(observation_point);
    bpf_map_update_elem(&retina_conntrack_map, &key, &new_value, BPF_ANY);
    return true;
}

/*
    * Create a new UDP connection.
    * @arg key The key to be used to create the new connection.
    * @arg flags The flags of the packet.
    * @arg observation_point The point in the network stack where the packet is observed.
*/
static __always_inline bool _ct_handle_udp_connection(struct ct_v4_key key, __u8 flags, enum obs_point observation_point) {
    struct ct_entry new_value;
    __builtin_memset(&new_value, 0, sizeof(struct ct_entry));
    __u64 now = bpf_mono_now();
    new_value.lifetime = now + CT_CONNECTION_LIFETIME_NONTCP;
    new_value.flags_seen_forward_dir = flags;
    new_value.last_report_forward_dir = now;
    new_value.traffic_direction = _ct_get_traffic_direction(observation_point);
    bpf_map_update_elem(&retina_conntrack_map, &key, &new_value, BPF_ANY);
    return true;
}

/*
    * Handle a TCP connection.
    * @arg key The key to be used to handle the connection.
    * @arg reverse_key The reverse key to be used to handle the connection.
    * @arg flags The flags of the packet.
    * @arg observation_point The point in the network stack where the packet is observed.
*/
static __always_inline bool _ct_handle_tcp_connection(struct ct_v4_key key, struct ct_v4_key reverse_key, __u8 flags, enum obs_point observation_point) {
    // Check if the packet is a SYN packet.
    if (flags & TCP_SYN) {
        // Create a new connection with a timeout of CT_SYN_TIMEOUT.
        return _ct_create_new_tcp_connection(key, flags, observation_point, CT_SYN_TIMEOUT);
    }

    // The packet is not a SYN packet and the connection corresponding to this packet is not found.
    // This might be because of an ongoing connection that started before Retina started tracking connections.
    // Therefore we would have missed the SYN packet. A conntrack entry will be created with best effort.
    struct ct_entry new_value;
    __builtin_memset(&new_value, 0, sizeof(struct ct_entry));
    __u64 now = bpf_mono_now();
    new_value.lifetime = now + CT_CONNECTION_LIFETIME_TCP;
    new_value.is_closing = (flags & (TCP_FIN | TCP_RST)) ? 1 : 0;
    new_value.traffic_direction = _ct_get_traffic_direction(observation_point);

    // Check for ACK flag. If the ACK flag is set, the packet is considered as a reply packet.
    if (flags & TCP_ACK) {
        new_value.flags_seen_reply_dir = flags;
        new_value.last_report_reply_dir = now;
        bpf_map_update_elem(&retina_conntrack_map, &reverse_key, &new_value, BPF_ANY);
    } else { // Otherwise, the packet is considered as a forward packet.
        new_value.flags_seen_forward_dir = flags;
        new_value.last_report_forward_dir = now;
        bpf_map_update_elem(&retina_conntrack_map, &key, &new_value, BPF_ANY);
    }
    return true;
}

/*
    * Handle a new connection.
    * @arg key The key to be used to handle the connection.
    * @arg reverse_key The reverse key to be used to handle the connection.
    * @arg flags The flags of the packet.
    * @arg observation_point The point in the network stack where the packet is observed.
*/
static __always_inline bool _ct_handle_new_connection(struct ct_v4_key key, struct ct_v4_key reverse_key, __u8 flags, enum obs_point observation_point) {
    // Check what kind of protocol the packet is.
    switch (key.proto) {
        case IPPROTO_TCP:
            return _ct_handle_tcp_connection(key, reverse_key, flags, observation_point);
        case IPPROTO_UDP:
            return _ct_handle_udp_connection(key, flags, observation_point);
        default:
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
static __always_inline bool _ct_should_report_packet(__u8 flags, struct ct_entry *entry, enum ct_packet_dir direction, __u8 protocol) {
    // Check for null parameters.
    if (!entry) {
        return false;
    }

    __u32 now = bpf_mono_now();
    __u32 lifetime = READ_ONCE(entry->lifetime);
    __u8 seen_flags;
    __u32 last_report;
    if (direction == CT_FORWARD) {
        seen_flags = READ_ONCE(entry->flags_seen_forward_dir);
        last_report = READ_ONCE(entry->last_report_forward_dir);
    } else {
        seen_flags = READ_ONCE(entry->flags_seen_reply_dir);
        last_report = READ_ONCE(entry->last_report_reply_dir);
    }
    // OR the seen flags with the new flags.
    flags |= seen_flags;

    // Check if the connection timed out of if it is a TCP connection and FIN or RST flags are set.
    if (now >= lifetime || (protocol == IPPROTO_TCP && flags & (TCP_FIN | TCP_RST))) {
        // The connection is closing or closed. Mark the connection as closing. Update the flags seen and last report time.
        WRITE_ONCE(entry->is_closing, 1);
        if (direction == CT_FORWARD) {
            WRITE_ONCE(entry->flags_seen_forward_dir, flags);
            WRITE_ONCE(entry->last_report_forward_dir, now);
        } else {
            WRITE_ONCE(entry->flags_seen_reply_dir, flags);
            WRITE_ONCE(entry->last_report_reply_dir, now);
        }
        return true; // Report the last packet received.
    }
    // Update the lifetime of the connection.
    if (protocol == IPPROTO_TCP) {
        WRITE_ONCE(entry->lifetime, now + CT_CONNECTION_LIFETIME_TCP);
    } else {
        WRITE_ONCE(entry->lifetime, now + CT_CONNECTION_LIFETIME_NONTCP);
    }
    // We will only report this packet iff a new flag is seen for the given direction or the report interval has passed.
    if (flags != seen_flags || now - last_report >= CT_REPORT_INTERVAL) {
        if (direction == CT_FORWARD) {
            WRITE_ONCE(entry->flags_seen_forward_dir, flags);
            WRITE_ONCE(entry->last_report_forward_dir, now);
        } else {
            WRITE_ONCE(entry->flags_seen_reply_dir, flags);
            WRITE_ONCE(entry->last_report_reply_dir, now);
        }
        return true;
    }
    return false;
}

/**
    * Process a packet and update the connection tracking map.
    * @arg key The key to be used to lookup the connection in the map.
    * @arg flags The flags of the packet.
    * @arg observation_point The point in the network stack where the packet is observed.
    * Returns true if the packet should be report to userspace. False otherwise.
**/
static __always_inline __attribute__((unused)) bool ct_process_packet(struct ct_v4_key key, __u8 flags, enum obs_point observation_point) {    
    // Lookup the connection in the map.
    struct ct_entry *entry = bpf_map_lookup_elem(&retina_conntrack_map, &key);

    // If the connection is found in the forward direction, update the connection.
    if (entry) {
        return _ct_should_report_packet(flags, entry, CT_FORWARD, key.proto);
    }
    
    // The connection is not found in the forward direction. Check the reply direction.
    struct ct_v4_key reverse_key;
    __builtin_memset(&reverse_key, 0, sizeof(struct ct_v4_key));
    reverse_key.src_ip = key.dst_ip;
    reverse_key.dst_ip = key.src_ip;
    reverse_key.src_port = key.dst_port;
    reverse_key.dst_port = key.src_port;
    reverse_key.proto = key.proto;

    // Lookup the connection in the map based on the reverse key.
    entry = bpf_map_lookup_elem(&retina_conntrack_map, &reverse_key);

    // If the connection is found based on the reverse key, meaning that the packet is a reply packet to an existing connection.
    if (entry) {
        return _ct_should_report_packet(flags, entry, CT_REPLY, key.proto);
    }

    // If the connection is still not found, the connection is new.
    return _ct_handle_new_connection(key, reverse_key, flags, observation_point);
}

/**
 * Check if a packet is a reply packet to a connection.
 * @arg key The key to be used to check if the packet is a reply packet.
 */
static __always_inline __attribute__((unused)) bool ct_is_reply_packet(struct ct_v4_key key) {    
    // Lookup the connection in the map.
    struct ct_entry *entry = bpf_map_lookup_elem(&retina_conntrack_map, &key);
    if (entry) {
        // We return false here because we found the connection in the forward direction
        // meaning that the packet is coming from the initiator of the connection and therefore not a reply packet.
        return false;
    } else {
        return true;
    }
}

/**
 * Get the traffic direction of a connection.
 * @arg key The key to be used to get the traffic direction of the connection.
 */
static __always_inline __attribute__((unused)) enum ct_traffic_dir ct_get_traffic_direction(struct ct_v4_key key) {
    // Lookup the connection in the map.
    struct ct_entry *entry = bpf_map_lookup_elem(&retina_conntrack_map, &key);
    if (entry) {
        return entry->traffic_direction;
    }
    // Construct the reverse key.
    struct ct_v4_key reverse_key;
    __builtin_memset(&reverse_key, 0, sizeof(struct ct_v4_key));
    reverse_key.src_ip = key.dst_ip;
    reverse_key.dst_ip = key.src_ip;
    reverse_key.src_port = key.dst_port;
    reverse_key.dst_port = key.src_port;
    reverse_key.proto = key.proto;
    // Lookup the connection in the map based on the reverse key.
    entry = bpf_map_lookup_elem(&retina_conntrack_map, &reverse_key);
    if (entry) {
        return entry->traffic_direction;
    }
    return TRAFFIC_DIRECTION_UNKNOWN;
}
