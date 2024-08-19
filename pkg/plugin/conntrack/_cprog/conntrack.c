// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "compiler.h"
#include "conntrack.h"


struct tcpmetadata {
	__u32 seq; // TCP sequence number
	__u32 ack_num; // TCP ack number
	// TCP flags.
	__u16 syn;
	__u16 ack;
	__u16 fin;
	__u16 rst;
	__u16 psh;
	__u16 urg;
	__u32 tsval; // TCP timestamp value
	__u32 tsecr; // TCP timestamp echo reply
};

struct packet
{
	// 5 tuple.
	__u32 src_ip;
	__u32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
	__u8 proto;
	struct tcpmetadata tcp_metadata; // TCP metadata
	enum obs_point observation_point; // 
	enum ct_traffic_dir traffic_direction; // 
	bool is_reply; // 0 -> FALSE, 1 -> TRUE
	__u64 ts; // timestamp in nanoseconds
	__u64 bytes; // packet size in bytes
};


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
struct ct_value {
    /* 
        * lifetime represents the time when the connection should be timed out.
    */
    __u32 lifetime;
    /*
        * traffic_direction represents the direction of the traffic of a connection in relation to the host
    */
    enum ct_traffic_dir traffic_direction;
    /*
        * flags_seen_*_dir represents the flags seen in the forward and reply direction.
    */
    __u8  flags_seen_forward_dir;
    __u8  flags_seen_reply_dir;
    /*
        * last_report_*_dir represents the last time a packet event was reported in the forward and reply direction.
    */
    __u32 last_report_forward_dir;
    __u32 last_report_reply_dir;
    /*
        * is_closing represents whether the connection is closing.
    */
    __u16 is_closing;
};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, struct ct_v4_key);
    __type(value, struct ct_value);
    __uint(max_entries, CT_MAP_SIZE);
} retina_conntrack_map SEC(".maps");

/**
     * Check if a TCP packet should be reported to userspace.
     * @arg flags The flags of the packet.
     * @arg value The value of the connection.
     * @arg direction The direction of the packet in relation to the connection.
     * Returns true if the packet should be reported to userspace. False otherwise.
 */
static __always_inline bool _should_report_tcp_packet(__u8 flags, struct ct_value *value, enum ct_packet_dir direction) {
    // Check for null parameters.
    if (!value) {
        return false;
    }

    __u32 now = bpf_mono_now();
    __u32 lifetime = READ_ONCE(value->lifetime);
    __u8 seen_flags;
    __u32 last_report;
    if (direction == CT_FORWARD) {
        seen_flags = READ_ONCE(value->flags_seen_forward_dir);
        last_report = READ_ONCE(value->last_report_forward_dir);
    } else {
        seen_flags = READ_ONCE(value->flags_seen_reply_dir);
        last_report = READ_ONCE(value->last_report_reply_dir);
    }
    // OR the seen flags with the new flags.
    flags |= seen_flags;

    // Check if the connection timed out or closed.
    if (now >= lifetime || flags & (TCP_FIN | TCP_RST)) {
        // The connection is closing or closed. Mark the connection as closing. Update the flags seen and last report time.
        WRITE_ONCE(value->is_closing, 1);
        if (direction == CT_FORWARD) {
            WRITE_ONCE(value->flags_seen_forward_dir, flags);
            WRITE_ONCE(value->last_report_forward_dir, now);
        } else {
            WRITE_ONCE(value->flags_seen_reply_dir, flags);
            WRITE_ONCE(value->last_report_reply_dir, now);
        }
        return true; // Report the last packet received.
    }
    // Update the lifetime of the connection.
    WRITE_ONCE(value->lifetime, now + CT_CONNECTION_LIFETIME_TCP);
    // We will only report this packet iff a new flag is seen for the given direction or the report interval has passed.
    if (flags != seen_flags || now - last_report >= CT_REPORT_INTERVAL) {
        if (direction == CT_FORWARD) {
            WRITE_ONCE(value->flags_seen_forward_dir, flags);
            WRITE_ONCE(value->last_report_forward_dir, now);
        } else {
            WRITE_ONCE(value->flags_seen_reply_dir, flags);
            WRITE_ONCE(value->last_report_reply_dir, now);
        }
        return true;
    }
    return false;
}
/**
     * Check if a UDP packet should be reported to userspace.
     * @arg value The value of the connection.
     * Returns true if the packet should be reported to userspace. False otherwise.
 */
static __always_inline bool _should_report_udp_packet(__u8 flags, struct ct_value *value, enum ct_packet_dir direction) {
    // Check for null parameters.
    if (!value) {
        return false;
    }

    __u32 now = bpf_mono_now();
    __u32 lifetime = READ_ONCE(value->lifetime);
    __u8 seen_flags;
    __u32 last_report;
    if (direction == CT_FORWARD) {
        seen_flags = READ_ONCE(value->flags_seen_forward_dir);
        last_report = READ_ONCE(value->last_report_forward_dir);
    } else {
        seen_flags = READ_ONCE(value->flags_seen_reply_dir);
        last_report = READ_ONCE(value->last_report_reply_dir);
    }
    // OR the seen flags with the new flags.
    flags |= seen_flags;

    // Check if the connection timed out or closed.
    if (now >= lifetime) {
        // The connection is closing or closed. Mark the connection as closing. Update the flags seen and last report time.
        WRITE_ONCE(value->is_closing, 1);
        if (direction == CT_FORWARD) {
            WRITE_ONCE(value->last_report_forward_dir, now);
        } else {
            WRITE_ONCE(value->last_report_reply_dir, now);
        }
        return true; // Report the last packet received.
    }
    // Update the lifetime of the connection.
    WRITE_ONCE(value->lifetime, now + CT_CONNECTION_LIFETIME_NONTCP);
    // We will only report this packet iff a new flag is seen for the given direction or the report interval has passed.
    if (flags != seen_flags || now - last_report >= CT_REPORT_INTERVAL) {
        if (direction == CT_FORWARD) {
            WRITE_ONCE(value->flags_seen_forward_dir, flags);
            WRITE_ONCE(value->last_report_forward_dir, now);
        } else {
            WRITE_ONCE(value->flags_seen_reply_dir, flags);
            WRITE_ONCE(value->last_report_reply_dir, now);
        }
        return true;
    }
    return false;
}

/**
    * Process a packet with the connection tracking logic.
    * @arg *p Pointer to the packet being processed.
    * @arg flags Flags seen on the packet.
    * @arg observation_point The point in the networking stack where the packet is observed.
    * Returns true if the packet should be report to userspace. False otherwise.
**/
static __always_inline __attribute__((unused)) bool ct_process_packet(struct packet *p, __u8 flags, enum obs_point observation_point) {
    if (!p) {
        return false;
    }
    // Create a new key for the forward direction.
    struct ct_v4_key key;
    __builtin_memset(&key, 0, sizeof(struct ct_v4_key));
    key.src_ip = p->src_ip;
    key.dst_ip = p->dst_ip;
    key.src_port = p->src_port;
    key.dst_port = p->dst_port;
    key.proto = p->proto;
    // Lookup the connection in the map.
    struct ct_value *value = bpf_map_lookup_elem(&retina_conntrack_map, &key);

    // If the connection is not found based on given packet, there are a few possibilities:
    // 1. The connection is new. This connection is either originated from the endpoint or destined to the endpoint.
    // 2. The packet belong to an existing connection but in the reply direction.
    if (!value) { // The connection is not found in the forward direction. Check the reply direction.
        // Create a new key for the reply direction.
        struct ct_v4_key reverse_key;
        __builtin_memset(&reverse_key, 0, sizeof(struct ct_v4_key));
        reverse_key.src_ip = key.dst_ip;
        reverse_key.dst_ip = key.src_ip;
        reverse_key.src_port = key.dst_port;
        reverse_key.dst_port = key.src_port;
        reverse_key.proto = key.proto;
        // Lookup the connection in the map based on the reverse key.
        value = bpf_map_lookup_elem(&retina_conntrack_map, &reverse_key);
        // If the connection is still not found, the connection is new.
        if (!value) {
            // Check what kind of protocol the packet is.
            switch(key.proto) {
                case IPPROTO_TCP: {
                    // Check if the packet is a SYN packet.
                    if (flags & TCP_SYN) {
                        // Create a new connection.
                        struct ct_value new_value;
                        __builtin_memset(&new_value, 0, sizeof(struct ct_value));
                        // Set the lifetime of the connection. Since this is a new connection, we will set the lifetime to SYN_TIMEOUT.
                        __u64 now = bpf_mono_now();
                        new_value.lifetime = now + CT_SYN_TIMEOUT;
                        new_value.flags_seen_forward_dir = flags;
                        new_value.last_report_forward_dir = now;
                        new_value.last_report_reply_dir = 0;
                        new_value.is_closing = 0;
                        // Set the traffic direction of the connection depening on the observation point.
                        if (observation_point == FROM_ENDPOINT) {
                            new_value.traffic_direction = TRAFFIC_DIRECTION_EGRESS;
                        } else if (observation_point == TO_ENDPOINT) {
                            new_value.traffic_direction = TRAFFIC_DIRECTION_INGRESS;
                        } else {
                            new_value.traffic_direction = TRAFFIC_DIRECTION_UNKNOWN;
                        }
                        bpf_map_update_elem(&retina_conntrack_map, &key, &new_value, BPF_ANY);
                        // Update the packet.
                        p->is_reply = false;
                        p->traffic_direction = new_value.traffic_direction;
                        return true;
                    } else {
                        // The packet is not a SYN packet and the connection corresponding to this packet is not found.
                        // This might be because of an ongoing connection that started before Retina started tracking connections.
                        // Therefore we would have missed the SYN packet. A conntrack entry will be created with best effort.
                        struct ct_value new_value;
                        __builtin_memset(&new_value, 0, sizeof(struct ct_value));
                        __u64 now = bpf_mono_now();
                        new_value.lifetime = now + CT_CONNECTION_LIFETIME_TCP;
                        new_value.is_closing = 0;
                        // Check for FIN and RST flags. If the connection is closing, mark the connection as closing.
                        if (flags & (TCP_FIN | TCP_RST)) {
                            new_value.is_closing = 1;
                        }
                        if (observation_point == FROM_ENDPOINT) {
                            new_value.traffic_direction = TRAFFIC_DIRECTION_EGRESS;
                        } else if (observation_point == TO_ENDPOINT) {
                            new_value.traffic_direction = TRAFFIC_DIRECTION_INGRESS;
                        } else {
                            new_value.traffic_direction = TRAFFIC_DIRECTION_UNKNOWN;
                        }
                        // Check for ACK flag. If the ACK flag is set, the packet is considered as a reply packet.
                        if (flags & TCP_ACK) {
                            new_value.flags_seen_reply_dir = flags;
                            new_value.last_report_forward_dir = 0;
                            new_value.last_report_reply_dir = now;
                            bpf_map_update_elem(&retina_conntrack_map, &reverse_key, &new_value, BPF_ANY);
                            p->is_reply = true;
                        } else {
                            new_value.flags_seen_forward_dir = flags;
                            new_value.last_report_forward_dir = now;
                            new_value.last_report_reply_dir = 0;
                            bpf_map_update_elem(&retina_conntrack_map, &key, &new_value, BPF_ANY);
                            p->is_reply = false;
                        }
                        p->traffic_direction = new_value.traffic_direction;
                        return true;
                    }
                }
                case IPPROTO_UDP: {
                    // Create a new connection.
                    struct ct_value new_value;
                    __builtin_memset(&new_value, 0, sizeof(struct ct_value));
                    // Set the lifetime of the connection. Since this is a new connection, we will set the lifetime to CONNECTION_LIFETIME_NONTCP.
                    __u64 now = bpf_mono_now();
                    new_value.lifetime = now + CT_CONNECTION_LIFETIME_NONTCP;
                    new_value.flags_seen_forward_dir = flags;
                    new_value.last_report_forward_dir = now;
                    new_value.last_report_reply_dir = 0;
                    new_value.is_closing = 0;
                    if (observation_point == FROM_ENDPOINT) {
                        new_value.traffic_direction = TRAFFIC_DIRECTION_EGRESS;
                    } else if (observation_point == TO_ENDPOINT) {
                        new_value.traffic_direction = TRAFFIC_DIRECTION_INGRESS;
                    } else {
                        new_value.traffic_direction = TRAFFIC_DIRECTION_UNKNOWN;
                    }
                    bpf_map_update_elem(&retina_conntrack_map, &key, &new_value, BPF_ANY);
                    p->is_reply = false;
                    p->traffic_direction = new_value.traffic_direction;
                    return true;
                }
                default:
                    return false; // We are not interested in other protocols.
            }
        } else { // The connection is found based on the reverse key, meaning that the packet is a reply packet to an existing connection.
             switch(reverse_key.proto) {
                case IPPROTO_TCP:
                    if (_should_report_tcp_packet(flags, value, CT_REPLY)) {
                        // is_reply is true here because we found the connection in the reverse direction
                        // meaning that the packet is coming from the responder of the connection and therefore a reply packet.
                        p->is_reply = true;
                        p->traffic_direction = value->traffic_direction;
                        return true;
                    }
                    return false;
                case IPPROTO_UDP:
                    if (_should_report_udp_packet(flags, value, CT_REPLY)) {
                        p->is_reply = true;
                        p->traffic_direction = value->traffic_direction;
                        return true;
                    }
                    return false;
                default:
                    return false; // We are not interested in other protocols.
            }
        }
    } else { // The connection is found in the forward direction. Update the connection.
        switch(key.proto) {
                case IPPROTO_TCP:
                    if (_should_report_tcp_packet(flags, value, CT_FORWARD)) {
                        // is_reply is false here because we found the connection in the forward direction
                        // meaning that the packet is coming from the initiator of the connection and therefore not a reply packet.
                        p->is_reply = false;
                        p->traffic_direction = value->traffic_direction;
                        return true;
                    }
                    return false;
                case IPPROTO_UDP:
                    if (_should_report_udp_packet(flags, value, CT_FORWARD)) {
                        p->is_reply = false;
                        p->traffic_direction = value->traffic_direction;
                        return true;
                    }
                    return false;
                default:
                    return false; // We are not interested in other protocols.
            }
    }
    return false;
}
