//go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "packetforward.h"

char __license[] SEC("license") = "Dual MIT/GPL";

// Ref: https://elixir.bootlin.com/linux/latest/source/include/uapi/linux/if_packet.h#L26
#define PACKET_HOST		    0       // Incomming packets
#define PACKET_OUTGOING		4		// Outgoing packets

struct metric
{
    __u64 count;
    __u64 bytes;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_HASH);
    __uint(max_entries, 2);
    __type(key, key_type);
    __type(value, struct metric);
} retina_packetforward_metrics SEC(".maps");

SEC("socket1")
int socket_filter(struct __sk_buff *skb) {
    key_type mapKey; //0->incoming; 1->outgoing
    
    if (skb->pkt_type == PACKET_HOST) {
        mapKey = INGRESS_KEY;
    } else if (skb->pkt_type == PACKET_OUTGOING) {
        mapKey = EGRESS_KEY;
    } else {
        // Ignore multicast/broadcast.
        return 0;
    }
    
    // Get the packet size (in bytes) including headers.
    u64 packetSize = skb->len;

    struct metric *curMetric = bpf_map_lookup_elem(&retina_packetforward_metrics, &mapKey);
	if (!curMetric) {
        // Per CPU hashmap, hence no race condition here.
        struct metric initMetric;
        initMetric.count = 1;
        initMetric.bytes = packetSize;
        bpf_map_update_elem(&retina_packetforward_metrics, &mapKey, &initMetric, BPF_ANY);
    } else {
        // Atomic operation.
        curMetric->count++;
        curMetric->bytes += packetSize;
    }

    return 0;
}
