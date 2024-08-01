// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "conntrack.h"


/**
    * The structure representing an ipv4 connection tracking key in the connection tracking map.
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
struct ct_value {
    /* 
        * lifetime represents the time when the connection will be closed.
    */
    __u32 lifetime;
    /*
        * flags_seen represents the flags that have been seen in the connection.
    */
	__u8  flags_seen;
    /*
        * last_report represents the last time when a packet for this connection was reported to userspace.
    */
	__u32 last_report;
    /*
        * is_closing represents whether the connection is closing.
    */
    __u16 is_closing;

};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, struct ct_v4_key);
    __type(value, struct ct_value);
    __uint(max_entries, 262144);
    __uint(pinning, LIBBPF_PIN_BY_NAME); // Pinned to /sys/fs/bpf.
} retina_conntrack_map SEC(".maps");

/**
    * Update the connection tracking map.
    * @arg key The key to be used to update the connection in the map.
    * @arg value The value to be updated in the map.
 **/
static __always_inline void _update_conntrack_map(struct ct_v4_key *key, struct ct_value *value) {
    // Check for null parameters.
    if (!key || !value) {
        return;
    }
    bpf_map_update_elem(&retina_conntrack_map, key, value, BPF_ANY);
}

/**
     * Check if a TCP packet should be reported to userspace.
     * @arg flags The flags of the packet.
     * @arg value The value of the connection.
     * Returns true if the packet should be reported to userspace. False otherwise.
 */
static __always_inline bool _should_report_tcp_packet(__u8 flags, struct ct_value *value) {
    // Check for null parameters.
    if (!value) {
        return false;
    }

    __u32 now = bpf_mono_now();
    __u32 lifetime = value->lifetime;
    // Check if the connection timed out or closed.
    if (now >= lifetime || flags & (TCP_FIN | TCP_RST)) {
        // The connection is closing or closed. Mark the connection as closing.
        value->is_closing = 1;
        return true; // Report the last packet received.
    }
    // Update the lifetime of the connection.
    value->lifetime = now + CT_CONNECTION_LIFETIME_TCP;
    __u8 seen_flags = value->flags_seen;
    __u32 last_report = value->last_report;
    // OR the seen flags with the new flags.
    flags |= seen_flags;
    // We will only report this packet iff a new flag is seen or the report interval has passed.
    if (flags != seen_flags || now - last_report >= CT_REPORT_INTERVAL) {
        value->flags_seen = flags;
        value->last_report = now;
        return true;
    }
    return false;
}
/**
     * Check if a UDP packet should be reported to userspace.
     * @arg value The value of the connection.
     * Returns true if the packet should be reported to userspace. False otherwise.
 */
static __always_inline bool _should_report_udp_packet(struct ct_value *value) {
    // Check for null parameters.
    if (!value) {
        return false;
    }
    __u32 now = bpf_mono_now();
    __u32 lifetime = value->lifetime;
    // Check if the connection timed out.
    if (now >= lifetime) {
        return true;
    }
    // Update the lifetime of the connection.
    value->lifetime = now + CT_CONNECTION_LIFETIME_NONTCP;
    __u32 last_report = value->last_report;
    // We will only report this packet if the report interval has passed.
    if (now - last_report >= CT_REPORT_INTERVAL) {
        value->last_report = now;
        return true;
    }
    return false;
}


/** 
    * Replicate the content of a ct_v4_key.
    * @arg new_key The new key to be replicated to.
    * @arg key The key to be replicated.
**/
static __always_inline void replicate_ct_v4_key(struct ct_v4_key *new_key, const struct ct_v4_key *key) {
    __builtin_memset(new_key, 0, sizeof(struct ct_v4_key));
    new_key->src_ip = key->src_ip;
    new_key->dst_ip = key->dst_ip;
    new_key->src_port = key->src_port;
    new_key->dst_port = key->dst_port;
    new_key->proto = key->proto;
}

/**
    * Process a packet and update the connection tracking map.
    * @arg key The key to be used to lookup the connection in the map.
    * @arg flags The flags of the packet.
    * Returns true if the packet should be report to userspace. False otherwise.
**/
static __always_inline bool ct_process_packet(struct ct_v4_key *key, __u8 flags) {
    if (!key) {
        return false;
    }
    // Checking whether the key is null is not enough for the eBPF verifier.
    // We need to recreate the key to avoid the verifier error.
    struct ct_v4_key new_key;
    replicate_ct_v4_key(&new_key, key);
    
    // Lookup the connection in the map.
    struct ct_value *value = bpf_map_lookup_elem(&retina_conntrack_map, &new_key);

    // If the connection is not found based on given packet, there are a few possibilities:
    // 1. The connection is new. This connection is either originated from the endpoint or destined to the endpoint.
    // 2. The packet belong to an existing connection but in the reverse direction.
    if (!value) { // The connection is not found in the forward direction. Check the reverse direction.
        // Create a new key for the reverse direction.
        struct ct_v4_key reverse_key;
        __builtin_memset(&reverse_key, 0, sizeof(struct ct_v4_key));
        reverse_key.src_ip = new_key.dst_ip;
        reverse_key.dst_ip = new_key.src_ip;
        reverse_key.src_port = new_key.dst_port;
        reverse_key.dst_port = new_key.src_port;
        reverse_key.proto = new_key.proto;
        // Lookup the connection in the map based on the reverse key.
        value = bpf_map_lookup_elem(&retina_conntrack_map, &reverse_key);
        // If the connection is still not found, the connection is new.
        if (!value) {
            // Check what kind of protocol the packet is.
            switch(new_key.proto) {
                case IPPROTO_TCP: {
                    // Check if the packet is a SYN packet.
                    if (flags & TCP_SYN) {
                        // Create a new connection.
                        struct ct_value new_value;
                        __builtin_memset(&new_value, 0, sizeof(struct ct_value));
                        // Set the lifetime of the connection. Since this is a new connection, we will set the lifetime to SYN_TIMEOUT.
                        __u64 now = bpf_mono_now();
                        new_value.lifetime = now + CT_SYN_TIMEOUT;
                        new_value.flags_seen = flags;
                        new_value.last_report = now;
                        new_value.is_closing = 0;
                        bpf_map_update_elem(&retina_conntrack_map, &new_key, &new_value, BPF_ANY);
                        return true;
                    } else {
                        // The packet is not a SYN packet and the connection corresponding to this packet is not found.
                        // This might be because of an ongoing connection that started before Retina started tracking connections.
                        // Therefore we would have missed the SYN packet. We will ignore this packet.
                        return false;
                    }
                }
                case IPPROTO_UDP: {
                    // Create a new connection.
                    struct ct_value new_value;
                    __builtin_memset(&new_value, 0, sizeof(struct ct_value));
                    // Set the lifetime of the connection. Since this is a new connection, we will set the lifetime to CONNECTION_LIFETIME_NONTCP.
                    __u64 now = bpf_mono_now();
                    new_value.lifetime = now + CT_CONNECTION_LIFETIME_NONTCP;
                    new_value.flags_seen = flags;
                    new_value.last_report = now;
                    new_value.is_closing = 0;
                    bpf_map_update_elem(&retina_conntrack_map, &new_key, &new_value, BPF_ANY);
                    return true;
                }
                default:
                    return false; // We are not interested in other protocols.
            }
        } else { // The connection is found based on the reverse key. Update the connection.
             switch(reverse_key.proto) {
                case IPPROTO_TCP:
                    if (_should_report_tcp_packet(flags, value)) {
                        _update_conntrack_map(&reverse_key, value);
                        return true;
                    }
                    return false;
                case IPPROTO_UDP:
                    if (_should_report_udp_packet(value)) {
                        _update_conntrack_map(&reverse_key, value);
                        return true;
                    }
                    return false;
                default:
                    return false; // We are not interested in other protocols.
            }
        }
    } else { // The connection is found in the forward direction. Update the connection.
        switch(new_key.proto) {
                case IPPROTO_TCP:
                    if (_should_report_tcp_packet(flags, value)) {
                        _update_conntrack_map(&new_key, value);
                        return true;
                    }
                    return false;
                case IPPROTO_UDP:
                    if (_should_report_udp_packet(value)) {
                        _update_conntrack_map(&new_key, value);
                        return true;
                    }
                    return false;
                default:
                    return false; // We are not interested in other protocols.
            }
    }
    return false;
}