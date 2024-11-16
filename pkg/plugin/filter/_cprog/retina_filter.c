//go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"

struct mapKey {
        __u32 prefixlen;
        __u32 data;
};

struct {
        __uint(type, BPF_MAP_TYPE_LPM_TRIE);
        __type(key, struct mapKey);
        __type(value, __u8);
        __uint(map_flags, BPF_F_NO_PREALLOC);
        __uint(max_entries, 255);
	__uint(pinning, LIBBPF_PIN_BY_NAME); // Pinned to /sys/fs/bpf.
} retina_filter SEC(".maps");

// Returns 1 if the IP address is in the map, 0 otherwise.
bool lookup(__u32 ipaddr)
{
        struct mapKey key = {
                .prefixlen = 32,
                .data = ipaddr
        };

        if (bpf_map_lookup_elem(&retina_filter, &key))
                return true;
        return false;
}
