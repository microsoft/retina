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
 * Parses the TSval and TSecr values from the TCP options field. If sucessful
 * the TSval and TSecr values will be stored at tsval and tsecr (in network
 * byte order).
 * Returns 0 if sucessful and -1 on failure
 */
static int parse_tcp_ts(struct tcphdr *tcph, void *data_end, __u32 *tsval, __u32 *tsecr){
	// Get pointer to the start of the options fields in the TCP header
	// In a TCP header, the options field starts immediately after the header. 
	// The size of the TCP header is given by the doff field, which is the 
	// number of 32-bit words in the header.

	// Check if the options field is present
	// either the data_end is before the start of the options field or the options field is not present
	if ((void *)(tcph + 1) > data_end) || (tcph->doff * 4 <= sizeof(struct tcphdr)){
		return -1;
	}

	// Get pointer to the start of the options field
	// The options field starts immediately after the header
	void *opt_ptr = (void *)(tcph + 1);

	// Iterate through the options field to find the TSval and TSecr values
	// util we reach the end of the options field or the end of the packet
	while (opt_ptr < data_end) || (opt_ptr < (void *)(tcph + tcph->doff * 4)){
		// Check if the option is the end of the options field. The kind field should be 0
		if (*(__u8 *)opt_ptr == 0){
			break;
		}

		// Check if the option is a NOP. The kind field should be 1
		if (*(__u8 *)opt_ptr == 1){
			opt_ptr++;
			continue;
		}

		// Check if the option is the timestamp option. The kind field should be 8
		if (*(__u8 *)opt_ptr == 8){
			// Check if the option is the correct size. The timestamp option is 10 bytes long
			if ((opt_ptr + 10) > data_end){
				return -1;
			}

			// Check if the option is the correct format. Adding 1 to the pointer
			// will get us to the length field.
			if (*(__u8 *)(opt_ptr + 1) != 10){
				return -1;
			}

			// Get the TSval and TSecr values, assiging them to the tsval and tsecr pointers
			*tsval = bpf_ntohl(*(__u32 *)(opt_ptr + 2));
			*tsecr = bpf_ntohl(*(__u32 *)(opt_ptr + 6));

			return 0;
		}

		// For all other options, the length field is the next byte after the kind field
		// We need to add 1 to the pointer to get to the length field and then add the length
		// to the pointer to get to the next option
		opt_ptr += *(__u8 *)(opt_ptr + 1);
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
