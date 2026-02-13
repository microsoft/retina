// go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// DNS tracer eBPF program - captures DNS queries and responses
//
// Adapted from Inspektor Gadget's trace_dns gadget (Apache 2.0 License)
// https://github.com/inspektor-gadget/inspektor-gadget
// Copyright (c) The Inspektor Gadget authors

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_endian.h"

char __license[] SEC("license") = "Dual MIT/GPL";

// Ethernet and IP constants
#define ETH_P_IP 0x0800
#define ETH_P_IPV6 0x86DD
#define ETH_HLEN 14

// IP protocol constants
#define IPPROTO_TCP 6
#define IPPROTO_UDP 17

// IPv6 next header values
#define NEXTHDR_HOP 0
#define NEXTHDR_TCP 6
#define NEXTHDR_UDP 17
#define NEXTHDR_ROUTING 43
#define NEXTHDR_FRAGMENT 44
#define NEXTHDR_AUTH 51
#define NEXTHDR_NONE 59
#define NEXTHDR_DEST 60

// Packet types from linux/if_packet.h
#define PACKET_HOST 0	  // Incoming packets
#define PACKET_OUTGOING 4 // Outgoing packets

// DNS constants
#define DNS_PORT 53
#define DNS_MDNS_PORT 5353
#define DNS_QR_QUERY 0
#define DNS_QR_RESP 1

// Maximum ports to check
#define MAX_PORTS 16
const volatile __u16 dns_ports[MAX_PORTS] = {DNS_PORT, DNS_MDNS_PORT};
const volatile __u16 dns_ports_len = 2;

// Helper functions for loading bytes from sk_buff
// These are special BPF intrinsics for socket programs
unsigned long long load_byte(const void *skb,
							 unsigned long long off) asm("llvm.bpf.load.byte");
unsigned long long load_half(const void *skb,
							 unsigned long long off) asm("llvm.bpf.load.half");
unsigned long long load_word(const void *skb,
							 unsigned long long off) asm("llvm.bpf.load.word");

// DNS header flags - handles both little and big endian
union dnsflags {
	struct {
#if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
		__u8 rcode : 4;	 // Response code
		__u8 z : 3;		 // Reserved
		__u8 ra : 1;	 // Recursion available
		__u8 rd : 1;	 // Recursion desired
		__u8 tc : 1;	 // Truncation
		__u8 aa : 1;	 // Authoritative answer
		__u8 opcode : 4; // Kind of query
		__u8 qr : 1;	 // 0=query; 1=response
#else
		__u8 qr : 1;
		__u8 opcode : 4;
		__u8 aa : 1;
		__u8 tc : 1;
		__u8 rd : 1;
		__u8 ra : 1;
		__u8 z : 3;
		__u8 rcode : 4;
#endif
	};
	__u16 flags;
};

// DNS header structure
struct dnshdr {
	__u16 id;
	union dnsflags flags;
	__u16 qdcount; // Question count
	__u16 ancount; // Answer count
	__u16 nscount; // Authority records
	__u16 arcount; // Additional records
};

// DNS event structure - sent to userspace
struct dns_event {
	__u64 timestamp;  // Boot time in nanoseconds
	__u32 src_ip;	  // Source IPv4 address
	__u32 dst_ip;	  // Destination IPv4 address
	__u8 src_ip6[16]; // Source IPv6 address
	__u8 dst_ip6[16]; // Destination IPv6 address
	__u16 src_port;	  // Source port
	__u16 dst_port;	  // Destination port
	__u16 id;		  // DNS query ID
	__u16 qtype;	  // Query type (from first question)
	__u8 af;		  // Address family (4 or 6)
	__u8 proto;		  // Protocol (TCP=6, UDP=17)
	__u8 pkt_type;	  // Packet type (HOST=0, OUTGOING=4)
	__u8 qr;		  // Query(0) or Response(1)
	__u8 rcode;		  // Response code
	__u8 _pad;
	__u16 ancount;	// Answer count
	__u16 dns_off;	// DNS offset in packet
	__u16 data_len; // Total packet length
};

// Define const for bpf2go type generation
const struct dns_event *unused_dns_event __attribute__((unused));

// Perf event array for streaming events to userspace
struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} retina_dns_events SEC(".maps");

// Per-CPU array for temporary event storage (avoids stack size limits)
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, struct dns_event);
} tmp_dns_events SEC(".maps");

// Query latency tracking map (optional)
struct query_key {
	__u16 id;
	__u16 src_port;
	__u32 src_ip;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, struct query_key);
	__type(value, __u64);
	__uint(max_entries, 1024);
} retina_dns_latency SEC(".maps");

// Check if port is a DNS port
static __always_inline bool is_dns_port(__u16 port) {
#pragma unroll
	for (int i = 0; i < MAX_PORTS; i++) {
		if (i >= dns_ports_len)
			break;
		if (dns_ports[i] == port)
			return true;
	}
	return false;
}

SEC("socket1")
int retina_dns_filter(struct __sk_buff *skb) {
	struct dns_event *event;
	__u16 h_proto, sport, dport, l4_off, dns_off;
	__u8 proto;
	int zero = 0;

	// First pass: Quick filter to check if this is a DNS packet
	h_proto = load_half(skb, offsetof(struct ethhdr, h_proto));

	switch (h_proto) {
	case ETH_P_IP: {
		// Get IP protocol
		proto = load_byte(skb, ETH_HLEN + offsetof(struct iphdr, protocol));

		// Calculate L4 offset - account for variable IP header length
		__u8 ihl_byte = load_byte(skb, ETH_HLEN);
		__u8 ip_header_len = (ihl_byte & 0x0F) * 4;
		l4_off = ETH_HLEN + ip_header_len;
		break;
	}
	case ETH_P_IPV6: {
		// Get next header (protocol)
		proto = load_byte(skb, ETH_HLEN + offsetof(struct ipv6hdr, nexthdr));
		l4_off = ETH_HLEN + sizeof(struct ipv6hdr);

// Parse IPv6 extension headers (up to 6)
#pragma unroll
		for (int i = 0; i < 6; i++) {
			__u8 nextproto;

			// Stop if we found TCP or UDP
			if (proto == NEXTHDR_TCP || proto == NEXTHDR_UDP)
				break;

			nextproto = load_byte(skb, l4_off);

			switch (proto) {
			case NEXTHDR_FRAGMENT:
				l4_off += 8;
				break;
			case NEXTHDR_AUTH:
				l4_off += 4 * (load_byte(skb, l4_off + 1) + 2);
				break;
			case NEXTHDR_HOP:
			case NEXTHDR_ROUTING:
			case NEXTHDR_DEST:
				l4_off += 8 * (load_byte(skb, l4_off + 1) + 1);
				break;
			case NEXTHDR_NONE:
				return 0;
			default:
				return 0;
			}
			proto = nextproto;
		}
		break;
	}
	default:
		return 0;
	}

	// Check protocol is TCP or UDP
	if (proto != IPPROTO_UDP && proto != IPPROTO_TCP)
		return 0;

	// Extract ports (same offsets for UDP and TCP)
	sport = load_half(skb, l4_off + offsetof(struct udphdr, source));
	dport = load_half(skb, l4_off + offsetof(struct udphdr, dest));

	// Early exit if not DNS port
	if (!is_dns_port(sport) && !is_dns_port(dport))
		return 0;

	// Calculate DNS offset
	switch (proto) {
	case IPPROTO_UDP:
		dns_off = l4_off + sizeof(struct udphdr);
		break;
	case IPPROTO_TCP: {
		// Get TCP header length (data offset field)
		__u8 doff_byte =
			load_byte(skb, l4_off + 12); // Offset to data offset field
		__u8 tcp_header_len = ((doff_byte >> 4) & 0x0F) * 4;

		// Skip if no data (control segment)
		dns_off = l4_off + tcp_header_len;
		if (skb->len <= dns_off)
			return 0;

		// DNS over TCP has 2-byte length prefix
		dns_off += 2;
		break;
	}
	default:
		return 0;
	}

	// Get event from per-CPU array (avoids stack size limits)
	event = bpf_map_lookup_elem(&tmp_dns_events, &zero);
	if (!event)
		return 0;

	// Initialize event with zeros for fields that might be skipped
	__builtin_memset(event, 0, sizeof(*event));

	// Fill in event data
	event->timestamp = bpf_ktime_get_boot_ns();
	event->data_len = skb->len;
	event->dns_off = dns_off;
	event->pkt_type = skb->pkt_type;
	event->proto = proto;
	event->src_port = sport;
	event->dst_port = dport;

	// Second pass: Extract IP addresses
	switch (h_proto) {
	case ETH_P_IP:
		event->af = 4;
		event->src_ip =
			load_word(skb, ETH_HLEN + offsetof(struct iphdr, saddr));
		event->dst_ip =
			load_word(skb, ETH_HLEN + offsetof(struct iphdr, daddr));
		// load_word converts to host byte order, convert back to network order
		event->src_ip = bpf_htonl(event->src_ip);
		event->dst_ip = bpf_htonl(event->dst_ip);
		break;
	case ETH_P_IPV6:
		event->af = 6;
		bpf_skb_load_bytes(skb, ETH_HLEN + offsetof(struct ipv6hdr, saddr),
						   event->src_ip6, 16);
		bpf_skb_load_bytes(skb, ETH_HLEN + offsetof(struct ipv6hdr, daddr),
						   event->dst_ip6, 16);
		break;
	}

	// Parse DNS header
	union dnsflags flags;
	flags.flags = load_half(skb, dns_off + offsetof(struct dnshdr, flags));
	event->qr = flags.qr;
	event->rcode = flags.rcode;
	event->id = load_half(skb, dns_off + offsetof(struct dnshdr, id));
	event->ancount =
		bpf_ntohs(load_half(skb, dns_off + offsetof(struct dnshdr, ancount)));

	// Try to get query type from first question (offset: DNS header + name + 2
	// bytes for qtype) This is simplified - full DNS name parsing is complex
	// We'll parse the full packet in userspace

	// Send event to userspace with the packet data appended
	__u64 skb_len = skb->len;
	bpf_perf_event_output(skb, &retina_dns_events,
						  skb_len << 32 | BPF_F_CURRENT_CPU, event,
						  sizeof(*event));

	return 0;
}
