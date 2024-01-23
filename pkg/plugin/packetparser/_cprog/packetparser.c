// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_endian.h"
#include "packetparser.h"
#include "retina_filter.c"
#include "dynamic.h"

char __license[] SEC("license") = "Dual MIT/GPL";


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
	direction dir; // 0 -> INGRESS, 1 -> EGRESS
	__u64 ts; // timestamp in nanoseconds
	__u64 bytes; // packet size in bytes
};

struct
{
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(max_entries, 16384);
} packetparser_events SEC(".maps");

// Define const variables to avoid warnings.
const struct packet *unused __attribute__((unused));

/*
 * Full credit to authors of pping_kern.c for the parse_tcp_ts function.
 * https://github.com/xdp-project/bpf-examples/blob/bc9df640cb9e5a541a7425ca2e66174ae22a18e3/pping/pping_kern.c#L316
 * 
 * Parses the TSval and TSecr values from the TCP options field. If sucessful
 * the TSval and TSecr values will be stored at tsval and tsecr (in network
 * byte order).
 * Returns 0 if sucessful and -1 on failure
 */
static int parse_tcp_ts(struct tcphdr *tcph, void *data_end, __u32 *tsval,
			__u32 *tsecr)
{
	int len = tcph->doff << 2;
	void *opt_end = (void *)tcph + len;
	__u8 *pos = (__u8 *)(tcph + 1); //Current pos in TCP options
	__u8 i, opt;
	volatile __u8
		opt_size; // Seems to ensure it's always read of from stack as u8

	if (tcph + 1 > data_end || len <= sizeof(struct tcphdr))
		return -1;
#pragma unroll //temporary solution until we can identify why the non-unrolled loop gets stuck in an infinite loop
	for (i = 0; i < MAX_TCP_OPTIONS; i++) {
		if (pos + 1 > opt_end || pos + 1 > data_end)
			return -1;

		opt = *pos;
		if (opt == 0) // Reached end of TCP options
			return -1;

		if (opt == 1) { // TCP NOP option - advance one byte
			pos++;
			continue;
		}

		// Option > 1, should have option size
		if (pos + 2 > opt_end || pos + 2 > data_end)
			return -1;
		opt_size = *(pos + 1);
		if (opt_size < 2) // Stop parsing options if opt_size has an invalid value
			return -1;

		// Option-kind is TCP timestap (yey!)
		if (opt == 8 && opt_size == 10) {
			if (pos + 10 > opt_end || pos + 10 > data_end)
				return -1;
			*tsval = bpf_ntohl(*(__u32 *)(pos + 2));
			*tsecr = bpf_ntohl(*(__u32 *)(pos + 6));
			return 0;
		}

		// Some other TCP option - advance option-length bytes
		pos += opt_size;
	}
	return -1;
}

// Function to parse the packet and send it to the perf buffer.
static void parse(struct __sk_buff *skb, direction d)
{
	struct packet p;
	__builtin_memset(&p, 0, sizeof(p));

	// Get current time in nanoseconds.
	p.ts = bpf_ktime_get_ns();
	
	p.dir = d;
	p.bytes = skb->len;

	void *data_end = (void *)(unsigned long long)skb->data_end;
	void *data = (void *)(unsigned long long)skb->data;

	// Check if the packet is not malformed.
	struct ethhdr *eth = data;
	if (data + sizeof(struct ethhdr) > data_end)
		return;

	// Check that this is an IP packet.
	if (bpf_ntohs(eth->h_proto) != ETH_P_IP)
		return;

	// Check if the packet is not malformed.
	struct iphdr *ip = data + sizeof(struct ethhdr);
	if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) > data_end)
		return;

	p.src_ip = ip->saddr;
	p.dst_ip = ip->daddr;
	p.proto = ip->protocol;

	// Check if the packet is of interest.
	#ifdef BYPASS_LOOKUP_IP_OF_INTEREST
	#if BYPASS_LOOKUP_IP_OF_INTEREST == 0
		if (!lookup(p.src_ip) && !lookup(p.dst_ip))
		{
			return;
		}
	#endif
	#endif
	// Get source and destination ports.
	if (ip->protocol == IPPROTO_TCP)
	{
		struct tcphdr *tcp = data + sizeof(struct ethhdr) + sizeof(struct iphdr);
		if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct tcphdr) > data_end)
			return;

		p.src_port = tcp->source;
		p.dst_port = tcp->dest;

		// Get TCP metadata.
		struct tcpmetadata tcp_metadata;
		__builtin_memset(&tcp_metadata, 0, sizeof(tcp_metadata));

		tcp_metadata.seq = tcp->seq;
		tcp_metadata.ack_num = tcp->ack_seq;
		tcp_metadata.syn = tcp->syn;
		tcp_metadata.ack = tcp->ack;
		tcp_metadata.fin = tcp->fin;
		tcp_metadata.rst = tcp->rst;
		tcp_metadata.psh = tcp->psh;
		tcp_metadata.urg = tcp->urg;

		p.tcp_metadata = tcp_metadata;

		// Get TSval/TSecr from TCP header.
		if (parse_tcp_ts(tcp, data_end, &tcp_metadata.tsval, &tcp_metadata.tsecr) == 0)
		{
			p.tcp_metadata = tcp_metadata;
		}
	}
	else if (ip->protocol == IPPROTO_UDP)
	{
		struct udphdr *udp = data + sizeof(struct ethhdr) + sizeof(struct iphdr);
		if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct udphdr) > data_end)
			return;

		p.src_port = udp->source;
		p.dst_port = udp->dest;
	}
	else
	{
		return;
	}

	bpf_perf_event_output(skb, &packetparser_events, BPF_F_CURRENT_CPU, &p, sizeof(p));
}

SEC("classifier_endpoint_ingress")
int endpoint_ingress_filter(struct __sk_buff *skb)
{
	// This is attached to the interface on the host side.
	// So ingress on host is egress on endpoint and vice versa.
	parse(skb, FROM_ENDPOINT);
	// Always return 0 to allow packet to pass.
	return 0;
}

SEC("classifier_endpoint_egress")
int endpoint_egress_filter(struct __sk_buff *skb)
{
	// This is attached to the interface on the host side.
	// So egress on host is ingress on endpoint and vice versa.
	parse(skb, TO_ENDPOINT);
	// Always return 0 to allow packet to pass.
	return 0;
}

SEC("classifier_host_ingress")
int host_ingress_filter(struct __sk_buff *skb)
{
	parse(skb, FROM_NETWORK);
	// Always return 0 to allow packet to pass.
	return 0;
}

SEC("classifier_host_egress")
int host_egress_filter(struct __sk_buff *skb)
{
	parse(skb, TO_NETWORK);
	// Always return 0 to allow packet to pass.
	return 0;
}
