//go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// tcpretrans — eBPF tracepoint program for TCP retransmission tracking.
//
// Attaches to the kernel tracepoint "tcp/tcp_retransmit_skb" which fires
// every time the kernel retransmits a TCP segment. For each event we
// extract the 5-tuple (src/dst IP + port, protocol), TCP state, and the
// TCP flags byte, then push the result to userspace via a perf buffer.
//
// Key design decisions:
//
//   Kernel version portability (CO-RE)
//     This program is compiled once and must load on any kernel from 5.8+.
//     (5.8 is the minimum because we use bpf_ktime_get_boot_ns, added in
//     commit 71d19214776e; the tracepoint itself exists since 4.15.)
//     The kernel can change struct layouts between versions, so we never
//     hard-code field offsets. Instead every field read goes through
//     BPF_CORE_READ / bpf_core_field_offset, which the loader (cilium/ebpf)
//     patches at load time to match the running kernel's actual layout.
//
//   Tracepoint struct rename (kernel 6.17)
//     Older kernels expose the tracepoint context as
//     "struct trace_event_raw_tcp_event_sk_skb" (shared with tcp_send_reset).
//     Kernel 6.17 (commit ad892e912b84) converted tcp_retransmit_skb to its
//     own TRACE_EVENT with a dedicated struct and an added "err" field.
//     We define both names using libbpf's ___flavor suffix convention — the
//     loader strips the suffix and matches whichever name exists in the
//     running kernel's BTF.
//     See: https://nakryiko.com/posts/bpf-core-reference-guide/#handling-incompatible-field-and-type-changes
//     bpf_core_type_exists() picks the right branch; the verifier removes
//     the dead one.
//
//   Reading IPs from the sock, not the tracepoint context
//     The tracepoint context has pre-copied IP fields (saddr/daddr), but
//     reading them would require type-branching for both struct names.
//     Instead we read IPs from the sock struct (sk->__sk_common.*), which
//     has the same data and a stable layout across all kernel versions.
//
// Ref: https://github.com/inspektor-gadget/inspektor-gadget/blob/c414fc1/gadgets/trace_tcpretrans/program.bpf.c

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_core_read.h"

char __license[] SEC("license") = "Dual MIT/GPL";

// Tracepoint context structs — two flavors for kernel version portability.
//
// The ___old / ___new suffixes are "CO-RE flavors": the loader strips
// everything after ___ when searching the kernel's BTF for a matching
// type. This avoids collisions with vmlinux.h (which may define either
// name depending on the kernel it was generated from).

// Kernel < 6.17: shared struct for tcp_retransmit_skb and tcp_send_reset.
struct trace_event_raw_tcp_event_sk_skb___old {
	struct trace_entry ent;
	const void *skbaddr;	// pointer to the retransmitted sk_buff
	const void *skaddr;	// pointer to the sock (TCP connection)
	int state;		// TCP state (ESTABLISHED, SYN_SENT, …)
	__u16 sport;		// source port
	__u16 dport;		// destination port
	__u16 family;		// address family (AF_INET / AF_INET6)
	__u8 saddr[4];		// source IPv4 (pre-copied from sock)
	__u8 daddr[4];		// dest IPv4
	__u8 saddr_v6[16];	// source IPv6
	__u8 daddr_v6[16];	// dest IPv6
	char __data[0];
};

// Kernel >= 6.17: per-tracepoint struct, adds an 'err' field.
struct trace_event_raw_tcp_retransmit_skb___new {
	struct trace_entry ent;
	const void *skbaddr;
	const void *skaddr;
	int state;
	__u16 sport;
	__u16 dport;
	__u16 family;
	__u8 saddr[4];
	__u8 daddr[4];
	__u8 saddr_v6[16];
	__u8 daddr_v6[16];
	int err;		// retransmission error code (new in 6.17)
	char __data[0];
};

struct tcpretrans_event {
	__u64 timestamp;  // boot time in nanoseconds (includes suspend)
	__u32 src_ip;	  // source IPv4 (network byte order)
	__u32 dst_ip;	  // destination IPv4 (network byte order)
	__u32 state;	  // TCP state enum
	__u16 src_port;	  // source port (host byte order)
	__u16 dst_port;	  // destination port (host byte order)
	__u8 src_ip6[16]; // source IPv6 address
	__u8 dst_ip6[16]; // destination IPv6 address
	__u8 tcpflags;	  // TCP flags byte (FIN/SYN/RST/PSH/ACK/URG/ECE/CWR)
	__u8 af;	  // address family shorthand (4 = IPv4, 6 = IPv6)
};

// Needed by bpf2go's -type flag to generate the Go struct.
const struct tcpretrans_event *unused_tcpretrans_event __attribute__((unused));

// Perf buffer map — one ring per CPU, sized by the Go loader.
struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} retina_tcpretrans_events SEC(".maps");

// Helper macro — reads the five tracepoint fields we need regardless of
// which struct flavor is active. The unused branch is eliminated by the
// verifier at load time.
#define READ_TP_FIELDS(tp_type, ctx, out_state, out_sport, out_dport,	\
		       out_sk, out_skb)					\
do {									\
	tp_type *tp = (tp_type *)(ctx);					\
	(out_state) = BPF_CORE_READ(tp, state);				\
	(out_sport) = BPF_CORE_READ(tp, sport);				\
	(out_dport) = BPF_CORE_READ(tp, dport);				\
	(out_sk)    = (const struct sock *)BPF_CORE_READ(tp, skaddr);	\
	(out_skb)   = (const void *)BPF_CORE_READ(tp, skbaddr);	\
} while (0)

SEC("tracepoint/tcp/tcp_retransmit_skb")
int retina_tcp_retransmit_skb(void *ctx) {
	struct tcpretrans_event event = {};

	event.timestamp = bpf_ktime_get_boot_ns();

	const struct sock *sk;
	const void *skbaddr;

	// Detect which tracepoint struct the running kernel uses.
	if (bpf_core_type_exists(struct trace_event_raw_tcp_retransmit_skb___new)) {
		READ_TP_FIELDS(struct trace_event_raw_tcp_retransmit_skb___new,
			       ctx, event.state, event.src_port,
			       event.dst_port, sk, skbaddr);
	} else {
		READ_TP_FIELDS(struct trace_event_raw_tcp_event_sk_skb___old,
			       ctx, event.state, event.src_port,
			       event.dst_port, sk, skbaddr);
	}

	// Read address family and IPs from the sock. These fields live in
	// sock_common (embedded in every sock) and are stable across versions.
	__u16 family = 0;
	BPF_CORE_READ_INTO(&family, sk, __sk_common.skc_family);

	if (family == 2) { // AF_INET
		event.af = 4;
		BPF_CORE_READ_INTO(&event.src_ip, sk,
				    __sk_common.skc_rcv_saddr);
		BPF_CORE_READ_INTO(&event.dst_ip, sk, __sk_common.skc_daddr);
	} else if (family == 10) { // AF_INET6
		event.af = 6;
		BPF_CORE_READ_INTO(event.src_ip6, sk,
				    __sk_common.skc_v6_rcv_saddr.in6_u.u6_addr8);
		BPF_CORE_READ_INTO(event.dst_ip6, sk,
				    __sk_common.skc_v6_daddr.in6_u.u6_addr8);
	} else {
		return 0;
	}

	// Read TCP flags from the skb's control buffer.
	//
	// Every sk_buff has a 48-byte scratch area called "cb" (char[48]).
	// For TCP, the kernel casts this area to "struct tcp_skb_cb" via the
	// TCP_SKB_CB() macro:
	//   #define TCP_SKB_CB(__skb) ((struct tcp_skb_cb *)&((__skb)->cb[0]))
	// See: https://github.com/torvalds/linux/blob/v6.12/include/net/tcp.h#L989
	//
	// We mirror that cast here. Neither &skb->cb[0] nor &tcb->tcp_flags
	// dereferences memory — they only compute addresses, with the compiler
	// recording CO-RE relocations for both field offsets. The single
	// bpf_probe_read_kernel() does the actual read. This is the same
	// pattern used by inspektor-gadget's trace_tcpretrans:
	// https://github.com/inspektor-gadget/inspektor-gadget/blob/c414fc1/gadgets/trace_tcpretrans/program.bpf.c#L158-L163
	if (skbaddr) {
		struct sk_buff *skb = (struct sk_buff *)skbaddr;
		struct tcp_skb_cb *tcb = (struct tcp_skb_cb *)&(skb->cb[0]);
		bpf_probe_read_kernel(&event.tcpflags, sizeof(event.tcpflags),
				      &tcb->tcp_flags);
	}

	bpf_perf_event_output(ctx, &retina_tcpretrans_events, BPF_F_CURRENT_CPU,
			      &event, sizeof(event));

	return 0;
}
