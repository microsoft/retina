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

// ── Packet read helpers ──
//
// All packet reads use bpf_skb_load_bytes, which copies raw bytes from
// the sk_buff into a local variable. Unlike the legacy BPF_LD_ABS
// intrinsics (load_byte/load_half), this works with BPF_PROG_TEST_RUN
// and is the recommended approach for modern BPF programs.
//
// bpf_skb_load_bytes does NOT convert byte order — multi-byte values
// are in network byte order (big-endian) and must be converted with
// bpf_ntohs/bpf_ntohl when used as host integers.

// Read a 1-byte value from the packet at the given offset.
static __always_inline int skb_load_byte(const struct __sk_buff *skb,
					 __u32 off, __u8 *out) {
	return bpf_skb_load_bytes(skb, off, out, 1);
}

// Read a 2-byte value from the packet at the given offset.
// Returns the value in host byte order.
static __always_inline int skb_load_half(const struct __sk_buff *skb,
					 __u32 off, __u16 *out) {
	__u16 val;
	int ret = bpf_skb_load_bytes(skb, off, &val, 2);
	if (ret == 0)
		*out = bpf_ntohs(val);
	return ret;
}

// DNS header structure (RFC 1035 §4.1.1).
// Used for sizeof and offsetof only — flag fields (QR, RCODE) are
// extracted manually to avoid bitfield portability issues.
struct dnshdr {
	__u16 id;
	__u16 flags;
	__u16 qdcount; // Question count
	__u16 ancount; // Answer count
	__u16 nscount; // Authority records
	__u16 arcount; // Additional records
};

// DNS event structure - sent to userspace.
// Fields are ordered by descending alignment (8 → 4 → 2 → 1) to avoid
// internal padding. The compiler adds 5 bytes of trailing padding to
// reach 8-byte struct alignment (required by the __u64 field).
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
	__u16 ancount;	  // Answer count
	__u16 dns_off;	  // DNS offset in packet
	__u16 data_len;	  // Total packet length
	__u8 af;		  // Address family (4 or 6)
	__u8 proto;		  // Protocol (TCP=6, UDP=17)
	__u8 pkt_type;	  // Packet type (HOST=0, OUTGOING=4)
	__u8 qr;		  // Query(0) or Response(1)
	__u8 rcode;		  // Response code
};

// Force bpf2go to generate a Go type for dns_event. Without this,
// bpf2go only generates types that appear in map definitions or
// program parameters. The -type flag references this symbol.
const struct dns_event *unused_dns_event __attribute__((unused));

// Perf event array for streaming events to userspace
struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} retina_dns_events SEC(".maps");

// BPF programs are limited to 512 bytes of stack (MAX_BPF_STACK in
// include/linux/filter.h: https://github.com/torvalds/linux/blob/master/include/linux/filter.h#L99),
// which is too small to hold a dns_event struct plus local variables.
// Instead we use a per-CPU array map with a single entry as heap-like
// scratch space — each CPU gets its own copy so there are no data races.
// Ref:
// https://github.com/inspektor-gadget/inspektor-gadget/blob/c414fc1/gadgets/trace_dns/program.bpf.c#L103-L109
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, struct dns_event);
} tmp_dns_events SEC(".maps");

// Check if port is standard DNS (53) or mDNS (5353).
static __always_inline bool is_dns_port(__u16 port) {
	return port == DNS_PORT || port == DNS_MDNS_PORT;
}

// Socket filter attached to a raw AF_PACKET socket (bound to all interfaces).
// Filters for DNS traffic (port 53/5353), extracts header metadata into a
// dns_event struct, and sends it to userspace via perf buffer with the raw
// packet appended for Go-side DNS payload parsing.
SEC("socket1")
int retina_dns_filter(struct __sk_buff *skb) {
	struct dns_event *event;
	__u16 h_proto, sport, dport, l4_off, dns_off;
	__u8 proto;
	int zero = 0;

	// Dedupe: drop TX-side observations. Every packet has at most one
	// RX-side observation in the host netns, so filtering out PACKET_OUTGOING
	// gives exactly-once counting across multi-interface hosts (pod veths,
	// bonded VFs, etc.). pkt_type reference:
	// https://github.com/torvalds/linux/blob/master/include/linux/etherdevice.h#L615
	if (skb->pkt_type == PACKET_OUTGOING)
		return 0;

	// First pass: Quick filter to check if this is a DNS packet
	if (skb_load_half(skb, offsetof(struct ethhdr, h_proto), &h_proto))
		return 0;

	switch (h_proto) {
	case ETH_P_IP: {
		// Get IP protocol
		if (skb_load_byte(skb, ETH_HLEN + offsetof(struct iphdr, protocol),
				  &proto))
			return 0;

		// Calculate L4 offset - account for variable IP header length
		__u8 ihl_byte;
		if (skb_load_byte(skb, ETH_HLEN, &ihl_byte))
			return 0;
		__u8 ip_header_len = (ihl_byte & 0x0F) * 4;
		l4_off = ETH_HLEN + ip_header_len;
		break;
	}
	case ETH_P_IPV6: {
		// Get next header (protocol)
		if (skb_load_byte(skb, ETH_HLEN + offsetof(struct ipv6hdr, nexthdr),
				  &proto))
			return 0;
		l4_off = ETH_HLEN + sizeof(struct ipv6hdr);

// Parse IPv6 extension headers (up to 6)
#pragma unroll
		for (int i = 0; i < 6; i++) {
			__u8 nextproto, ext_len;

			// Stop if we found TCP or UDP
			if (proto == NEXTHDR_TCP || proto == NEXTHDR_UDP)
				break;

			if (skb_load_byte(skb, l4_off, &nextproto))
				return 0;

			switch (proto) {
			case NEXTHDR_FRAGMENT:
				l4_off += 8;
				break;
			case NEXTHDR_AUTH:
				if (skb_load_byte(skb, l4_off + 1, &ext_len))
					return 0;
				l4_off += 4 * (ext_len + 2);
				break;
			case NEXTHDR_HOP:
			case NEXTHDR_ROUTING:
			case NEXTHDR_DEST:
				if (skb_load_byte(skb, l4_off + 1, &ext_len))
					return 0;
				l4_off += 8 * (ext_len + 1);
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
	if (skb_load_half(skb, l4_off + offsetof(struct udphdr, source), &sport))
		return 0;
	if (skb_load_half(skb, l4_off + offsetof(struct udphdr, dest), &dport))
		return 0;

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
		__u8 doff_byte;
		if (skb_load_byte(skb, l4_off + 12, &doff_byte))
			return 0;
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

	// Look up index 0 of the per-CPU array to get a pointer to this CPU's
	// scratch buffer. The map only has one entry (max_entries=1), so index 0
	// is the only valid key. This returns a pointer the verifier trusts for
	// bounded writes, unlike a raw stack allocation.
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

	// Extract IP addresses using bpf_skb_load_bytes for raw byte copy —
	// no byte-order conversion, so the Go side gets network-order bytes
	// that map directly to net.IP.
	switch (h_proto) {
	case ETH_P_IP:
		event->af = 4;
		bpf_skb_load_bytes(skb, ETH_HLEN + offsetof(struct iphdr, saddr),
						   &event->src_ip, 4);
		bpf_skb_load_bytes(skb, ETH_HLEN + offsetof(struct iphdr, daddr),
						   &event->dst_ip, 4);
		break;
	case ETH_P_IPV6:
		event->af = 6;
		bpf_skb_load_bytes(skb, ETH_HLEN + offsetof(struct ipv6hdr, saddr),
						   event->src_ip6, 16);
		bpf_skb_load_bytes(skb, ETH_HLEN + offsetof(struct ipv6hdr, daddr),
						   event->dst_ip6, 16);
		break;
	}

	// Bounds check: ensure DNS header (12 bytes) fits in packet
	if (skb->len < dns_off + sizeof(struct dnshdr))
		return 0;

	// Extract fields from the 12-byte DNS header (RFC 1035 §4.1.1):
	//
	//   Offset  Field
	//   0-1     ID        (transaction identifier)
	//   2       flags[0]  QR(1) | Opcode(4) | AA(1) | TC(1) | RD(1)
	//   3       flags[1]  RA(1) | Z(3) | RCODE(4)
	//   4-5     QDCOUNT
	//   6-7     ANCOUNT
	//   8-9     NSCOUNT
	//   10-11   ARCOUNT
	//
	// We read the flag bytes individually rather than using a C bitfield
	// struct because bitfield layout is compiler-dependent (bit ordering
	// varies between GCC and Clang, and between big/little-endian targets).
	// Manual shifts give us portable, predictable extraction.
	__u8 flags0, flags1;
	if (skb_load_byte(skb, dns_off + 2, &flags0))
		goto send;
	if (skb_load_byte(skb, dns_off + 3, &flags1))
		goto send;
	event->qr = (flags0 >> 7) & 1;    // QR is the high bit of flags[0]
	event->rcode = flags1 & 0x0F;      // RCODE is the low 4 bits of flags[1]
	skb_load_half(skb, dns_off + offsetof(struct dnshdr, id), &event->id);
	skb_load_half(skb, dns_off + offsetof(struct dnshdr, ancount),
		      &event->ancount);

	// ── Extract QTYPE (query type) from the first DNS question ──
	//
	// DNS question format (RFC 1035 §4.1.2):
	//
	//   [12-byte header]  ← already parsed above
	//   [QNAME]           ← variable-length, encoded as labels
	//   [QTYPE]           ← 2 bytes (e.g. 1=A, 28=AAAA)
	//   [QCLASS]          ← 2 bytes (usually 1=IN)
	//
	// QNAME is a sequence of length-prefixed labels ending with a zero byte:
	//   "kubernetes.default.svc." → [10]kubernetes [7]default [3]svc [0]
	//
	// We read each label's length byte, skip that many bytes, and repeat
	// until we hit the zero terminator. QTYPE sits right after it.
	//
	// The loop is bounded to 64 iterations for the BPF verifier (DNS names
	// can be at most 253 bytes / ~127 labels). If the packet is truncated
	// mid-name, we skip QTYPE and let the Go side extract it via gopacket.
	{
		__u16 qoff = dns_off + sizeof(struct dnshdr);
#pragma unroll
		for (int i = 0; i < 64; i++) {
			if (qoff >= skb->len)
				goto send;
			__u8 label_len;
			if (skb_load_byte(skb, qoff, &label_len))
				goto send;
			if (label_len == 0) {
				qoff += 1; // skip zero terminator
				break;
			}
			qoff += 1 + label_len; // skip length byte + label
		}
		if (qoff + 2 <= skb->len)
			skb_load_half(skb, qoff, &event->qtype);
	}

send:
	// Send the structured event + raw packet bytes to userspace in one call.
	//
	// bpf_perf_event_output writes `event` (sizeof(*event) bytes) into the
	// perf ring buffer, then optionally appends raw packet data from `skb`.
	//
	// The flags parameter encodes two things:
	//   - Lower 32 bits: BPF_F_CURRENT_CPU — target this CPU's ring buffer
	//   - Upper 32 bits: skb->len — number of packet bytes to append
	//
	// The Go side receives a single record.RawSample containing both:
	//   [ dns_event struct ][ raw packet (skb->len bytes) ]
	//
	// It splits at sizeof(dns_event) to get the structured metadata and the
	// raw DNS payload, which gopacket parses for the query name and answers.
	bpf_perf_event_output(skb, &retina_dns_events,
						  (__u64)skb->len << 32 | BPF_F_CURRENT_CPU, event,
						  sizeof(*event));

	return 0;
}
