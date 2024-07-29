// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"

struct ct_key {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8 protocol;
};

struct ct_value {
    __u64 timestamp;
    __u32 flags;
    __u8 isClosed;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct ct_key);
    __type(value, struct ct_value);
    __uint(max_entries, 4096);
    __uint(pinning, LIBBPF_PIN_BY_NAME); // Pinned to /sys/fs/bpf.
} retina_conntrack_map SEC(".maps");

/*
    * Process a packet and update the connection tracking map.
    * key: The key to be used to lookup the connection in the map.
    * flags: The flags of the packet.
    * Returns 0 on success.
*/
int ct_process_packet(struct ct_key *key, __u8 flags)
{
    // Check if key is not NULL.
    if (!key) {
        return 0;
    }

    // Reconstruct the key
    struct ct_key new_key;
    __builtin_memset(&new_key, 0, sizeof(new_key));
    new_key.src_ip = key->src_ip;
    new_key.dst_ip = key->dst_ip;
    new_key.src_port = key->src_port;
    new_key.dst_port = key->dst_port;
    new_key.protocol = key->protocol;

    struct ct_value *value = bpf_map_lookup_elem(&retina_conntrack_map, &new_key);
    if (!value) {
        // If the connection is not in the map, add it.
        struct ct_value new_value;
        __builtin_memset(&new_value, 0, sizeof(new_value));
        new_value.timestamp = bpf_ktime_get_ns();
        new_value.flags = flags;
        new_value.isClosed = 0;
        bpf_map_update_elem(&retina_conntrack_map, &new_key, &new_value, BPF_ANY);

    } else {
        // Check the protocol.
        switch (new_key.protocol) {
            case IPPROTO_TCP:
                // If FIN flag is set, set its state to closed.
                // Checking the least significant bit of tcp_flags since it is the FIN flag.
                if (flags & 0x01) {
                    value->isClosed = 1;
                }
                // Update seen flags.
                value->flags |= flags;
                break;
            case IPPROTO_UDP:
                // If the protocol is UDP, we only need to update the timestamp for now
                break;
            default:
                break;
        }
        // Update the timestamp.
        value->timestamp = bpf_ktime_get_ns();
    }
    return 0;
}

/*
    * Check if the flags in packet_flags have been seen before.
    * key: The key to be used to lookup the connection in the map.
    * packet_flags: The flags of the packet.
    * Returns true if all flags in packet_flags have been seen before, false otherwise.
*/
bool ct_check_flags(struct ct_key *key, __u32 packet_flags)
{
    // Check if key is not NULL.
    if (!key) {
        return false;
    }
    
    // Reconstruct the key
    struct ct_key new_key;
    __builtin_memset(&new_key, 0, sizeof(new_key));
    new_key.src_ip = key->src_ip;
    new_key.dst_ip = key->dst_ip;
    new_key.src_port = key->src_port;
    new_key.dst_port = key->dst_port;
    new_key.protocol = key->protocol;
        
    struct ct_value *value = bpf_map_lookup_elem(&retina_conntrack_map, &new_key);
    if (!value) {
        return false; // If the connection is not in the map, return false.
    }
    if ((value->flags & packet_flags) == packet_flags) {
        return true; // If all flags in packet_flags have been seen before, return true.
    }

    return false; // If not all flags in packet_flags have been seen before, return false.
}