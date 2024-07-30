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
    __uint(max_entries, 262144);
    __uint(pinning, LIBBPF_PIN_BY_NAME); // Pinned to /sys/fs/bpf.
} retina_conntrack_map SEC(".maps");

/*
    * Reconstruct a key from an existing key.
    * new_key: The key to be reconstructed.
    * key: The key to be used as a reference.
*/
static void reconstruct_key(struct ct_key *new_key, const struct ct_key *key) {
    __builtin_memset(new_key, 0, sizeof(*new_key));
    new_key->src_ip = key->src_ip;
    new_key->dst_ip = key->dst_ip;
    new_key->src_port = key->src_port;
    new_key->dst_port = key->dst_port;
    new_key->protocol = key->protocol;
}

/*
    * Lookup a connection in the connection tracking map. If the connection does not exist, create it.
    * key: The key to be used to lookup the connection in the map.
    * flags: The flags of the packet.
    * Returns the pointer to the value in the map.
*/
static struct ct_value* lookup_or_create_connection(struct ct_key *key, __u8 flags) {
    // Only process TCP packets
    if (key->protocol != IPPROTO_TCP) {
        return NULL;
    }
    struct ct_value *value = bpf_map_lookup_elem(&retina_conntrack_map, key);
    if (!value) {
        struct ct_value new_value;
        __builtin_memset(&new_value, 0, sizeof(new_value));
        new_value.timestamp = bpf_ktime_get_ns();
        new_value.flags = flags;
        new_value.isClosed = 0;
        bpf_map_update_elem(&retina_conntrack_map, key, &new_value, BPF_ANY);
        value = bpf_map_lookup_elem(&retina_conntrack_map, key);
    }
    return value;
}

/*
    * Update the connection state in the connection tracking map.
    * value: The value to be updated.
    * flags: The flags of the packet.
    * protocol: The protocol of the packet.
*/
static void update_connection_state(struct ct_value *value, __u8 flags, __u8 protocol) {
    switch (protocol) {
        case IPPROTO_TCP:
            // Check if the connection is closed or being reset.
            if (flags & (0x01 | 0x04)) {
                value->isClosed = 1;
            }
            value->flags |= flags;
            value->timestamp = bpf_ktime_get_ns();
            break;
        case IPPROTO_UDP:
            // Not implemented.
            break;
        default:
            break;
    }
}



/*
    * Process a packet and update the connection tracking map.
    * key: The key to be used to lookup the connection in the map.
    * flags: The flags of the packet.
    * Returns true if the packet should be output to userspace. False otherwise.
*/
bool ct_process_packet(struct ct_key *key, __u8 flags) {
    if (!key) {
        return false;
    }
    // Checking whether the key is null is not enough for the eBPF verifier.
    // We need to reconstruct the key to avoid the verifier error.
    struct ct_key new_key;
    reconstruct_key(&new_key, key);

    struct ct_value *value = lookup_or_create_connection(&new_key, flags);
    if (value) {
        bool is_new_tcp_flag = (new_key.protocol == IPPROTO_TCP) && ((value->flags & flags) != flags);
        update_connection_state(value, flags, new_key.protocol);
        return is_new_tcp_flag;
    }

    return false;
}