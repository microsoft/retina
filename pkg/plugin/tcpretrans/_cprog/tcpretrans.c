// go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// TCP retransmission tracer eBPF program
// Hooks into tcp_retransmit_skb tracepoint/kprobe to capture TCP retransmission
// events
//
// Based on tcpretrans from BCC (Apache 2.0 License)
// https://github.com/iovisor/bcc
// Copyright (c) 2016 Netflix, Inc.
// Author: Brendan Gregg

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_core_read.h"
#include "bpf_tracing.h"
#include "bpf_endian.h"

char __license[] SEC("license") = "Dual MIT/GPL";

// TCP retransmission event structure - sent to userspace
struct tcpretrans_event {
	__u64 timestamp; // Boot time in nanoseconds
	__u32 src_ip;	 // Source IPv4 address (network byte order)
	__u32 dst_ip;	 // Destination IPv4 address (network byte order)
	__u16 src_port;	 // Source port (host byte order)
	__u16 dst_port;	 // Destination port (host byte order)
	__u32 state;	 // TCP state
	__u8 tcpflags;	 // TCP flags from the retransmitted packet
	__u8 af;		 // Address family (4 or 6)
	__u8 _pad[2];
	__u8 src_ip6[16]; // Source IPv6 address
	__u8 dst_ip6[16]; // Destination IPv6 address
};

// Define const for bpf2go type generation
const struct tcpretrans_event *unused_tcpretrans_event __attribute__((unused));

// Perf event array for streaming events to userspace
struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} retina_tcpretrans_events SEC(".maps");

// TCP flag bit positions
#define TCP_FLAG_FIN 0x01
#define TCP_FLAG_SYN 0x02
#define TCP_FLAG_RST 0x04
#define TCP_FLAG_PSH 0x08
#define TCP_FLAG_ACK 0x10
#define TCP_FLAG_URG 0x20
#define TCP_FLAG_ECE 0x40
#define TCP_FLAG_CWR 0x80

static __always_inline void extract_tcp_info(struct sock *sk,
											 struct tcpretrans_event *event) {
	// Read address family
	__u16 family = 0;
	BPF_CORE_READ_INTO(&family, sk, __sk_common.skc_family);

	if (family == 2) { // AF_INET
		event->af = 4;
		BPF_CORE_READ_INTO(&event->src_ip, sk, __sk_common.skc_rcv_saddr);
		BPF_CORE_READ_INTO(&event->dst_ip, sk, __sk_common.skc_daddr);
	} else if (family == 10) { // AF_INET6
		event->af = 6;
		BPF_CORE_READ_INTO(&event->src_ip6, sk,
						   __sk_common.skc_v6_rcv_saddr.in6_u.u6_addr8);
		BPF_CORE_READ_INTO(&event->dst_ip6, sk,
						   __sk_common.skc_v6_daddr.in6_u.u6_addr8);
	} else {
		return;
	}

	// Read ports
	__u16 dport = 0;
	BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
	event->dst_port = bpf_ntohs(dport);

	__u16 sport = 0;
	BPF_CORE_READ_INTO(&sport, sk, __sk_common.skc_num);
	event->src_port = sport; // already in host byte order

	// Read TCP state
	BPF_CORE_READ_INTO(&event->state, sk, __sk_common.skc_state);
}

SEC("kprobe/tcp_retransmit_skb")
int BPF_KPROBE(retina_tcp_retransmit_skb, struct sock *sk,
			   struct sk_buff *skb) {
	if (!sk || !skb)
		return 0;

	struct tcpretrans_event event = {};
	event.timestamp = bpf_ktime_get_boot_ns();

	extract_tcp_info(sk, &event);

	// Skip if we couldn't determine the address family
	if (event.af == 0)
		return 0;

	// Read TCP flags from the skb's transport header
	// TCP flags byte is at offset 13 in the TCP header
	// (byte 12 = data offset + reserved, byte 13 = flags)
	// Cannot use BPF_CORE_READ_INTO on bit-fields, so read the raw byte
	char *head = NULL;
	__u16 trans_header = 0;
	BPF_CORE_READ_INTO(&head, skb, head);
	BPF_CORE_READ_INTO(&trans_header, skb, transport_header);

	if (head && trans_header) {
		// Read the flags byte (offset 13 in TCP header)
		__u8 flags_byte = 0;
		bpf_probe_read_kernel(&flags_byte, sizeof(flags_byte),
							  head + trans_header + 13);
		event.tcpflags = flags_byte;
	}

	bpf_perf_event_output(ctx, &retina_tcpretrans_events, BPF_F_CURRENT_CPU,
						  &event, sizeof(event));

	return 0;
}
