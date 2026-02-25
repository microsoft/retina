// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//
// Shared packet parsing logic used by both TC and TCX packetparser plugins.
// This header should be included after vmlinux.h, bpf_helpers.h, bpf_endian.h,
// packetparser.h, conntrack.c, conntrack.h, retina_filter.c, and dynamic.h.

#ifndef __PACKETPARSE_H__
#define __PACKETPARSE_H__

/*
 * Parses the TSval and TSecr values from the TCP options field. If sucessful
 * the TSval and TSecr values will be stored at tsval and tsecr (in network
 * byte order).
 * Returns 0 if sucessful and -1 on failure
 *
   +-------+-------+---------------------+---------------------+
   |Kind=8 |  10   | TS Value (TSval)    |TS Echo Reply (TSecr)|
   +-------+-------+---------------------+---------------------+
	  1       1               4                      4
 * Reference: 
 * - https://github.com/xdp-project/bpf-examples
 * - https://www.ietf.org/rfc/rfc9293.html
 * - https://www.rfc-editor.org/rfc/pdfrfc/rfc7323.txt.pdf
 * May explore using bpf_loop() in the future (kernel 5.17+)
*/
static int parse_tcp_ts(struct tcphdr *tcph, void *data_end, __u32 *tsval, __u32 *tsecr) {
	__u8 tcp_header_len = tcph->doff << 2;
	volatile __u8 opt_len;
	__u8 opt_kind, i;

	if (tcp_header_len <= sizeof(struct tcphdr)) {
		return -1;
	}

	__u8 *tcp_opt_end_ptr = (__u8 *)tcph + tcp_header_len;

	if ((__u8 *)tcph + 1 > (__u8 *)data_end) {
		return -1;
	}

	__u8 *tcp_options_cur_ptr = (__u8 *)(tcph + 1);

	#pragma unroll
	for (i = 0; i < MAX_TCP_OPTIONS_LEN; i++) {
		if (tcp_options_cur_ptr + 1 > (__u8 *)tcp_opt_end_ptr || tcp_options_cur_ptr + 1 > (__u8 *)data_end) {
			return -1;
		}
		opt_kind = *tcp_options_cur_ptr;
		switch (opt_kind) {
			case 0:
				return -1;
			case 1:
				tcp_options_cur_ptr++;
				continue;
			default:
				if (tcp_options_cur_ptr + 2 > tcp_opt_end_ptr || tcp_options_cur_ptr + 2 > (__u8 *)data_end) {
					return -1;
				}
				opt_len = *(tcp_options_cur_ptr + 1);
				if (opt_len < 2) {
					return -1;
				}
				if (opt_kind == 8 && opt_len == 10) {
					if (tcp_options_cur_ptr + 10 > tcp_opt_end_ptr || tcp_options_cur_ptr + 10 > (__u8 *)data_end) {
						return -1;
					}
					*tsval = bpf_ntohl(*(__u32 *)(tcp_options_cur_ptr + 2));
					*tsecr = bpf_ntohl(*(__u32 *)(tcp_options_cur_ptr + 6));

					return 0;
				}
				tcp_options_cur_ptr += opt_len;
		}
	}
	return -1;
}

/*
 * Parses a packet from an sk_buff and sends the parsed data to the perf event buffer.
 * `skb` is the socket buffer to parse.
 * `obs` is the observation point (e.g., FROM_ENDPOINT, TO_ENDPOINT, etc.).
 * `events_map` is a pointer to the perf event array map.
 */
static void parse(struct __sk_buff *skb, __u8 obs, void *events_map)
{
	struct packet p;
	__builtin_memset(&p, 0, sizeof(p));

	p.t_nsec = bpf_ktime_get_boot_ns();
	
	p.observation_point = obs;
	p.bytes = skb->len;

	void *data_end = (void *)(unsigned long long)skb->data_end;
	void *data = (void *)(unsigned long long)skb->data;

	struct ethhdr *eth = data;
	if (data + sizeof(struct ethhdr) > data_end)
		return;

	if (bpf_ntohs(eth->h_proto) != ETH_P_IP)
		return;

	struct iphdr *ip = data + sizeof(struct ethhdr);
	if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) > data_end)
		return;

	p.src_ip = ip->saddr;
	p.dst_ip = ip->daddr;
	p.proto = ip->protocol;

	#ifdef BYPASS_LOOKUP_IP_OF_INTEREST
	#if BYPASS_LOOKUP_IP_OF_INTEREST == 0
		if (!lookup(p.src_ip) && !lookup(p.dst_ip))
		{
			return;
		}
	#endif
	#endif

	if (ip->protocol == IPPROTO_TCP)
	{
		struct tcphdr *tcp = data + sizeof(struct ethhdr) + sizeof(struct iphdr);
		if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct tcphdr) > data_end)
			return;

		p.src_port = tcp->source;
		p.dst_port = tcp->dest;

		struct tcpmetadata tcp_metadata;
		__builtin_memset(&tcp_metadata, 0, sizeof(tcp_metadata));

		p.flags = (tcp->fin << 0) | (tcp->syn << 1) | (tcp->rst << 2) | (tcp->psh << 3) | (tcp->ack << 4) | (tcp->urg << 5) | (tcp->ece << 6) | (tcp->cwr << 7);

		tcp_metadata.seq = tcp->seq;
		tcp_metadata.ack_num = tcp->ack_seq;
		p.tcp_metadata = tcp_metadata;

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

		p.flags = 1;
	}
	else
	{
		return;
	}

	#ifdef ENABLE_CONNTRACK_METRICS
		struct conntrackmetadata conntrack_metadata;
		__builtin_memset(&conntrack_metadata, 0, sizeof(conntrack_metadata));
		p.conntrack_metadata = conntrack_metadata;
	#endif // ENABLE_CONNTRACK_METRICS

    #ifdef DATA_AGGREGATION_LEVEL

	bool sampled __attribute__((unused));
	sampled = true;
	
	#ifdef DATA_SAMPLING_RATE
	    u32 rand __attribute__((unused));
		rand = bpf_get_prandom_u32();
		if (rand >= UINT32_MAX / DATA_SAMPLING_RATE) {
			sampled = false;
		}
	#endif
	
	struct packetreport report __attribute__((unused));
	report = ct_process_packet(&p, obs, sampled);

	#if DATA_AGGREGATION_LEVEL == DATA_AGGREGATION_LEVEL_LOW
		p.previously_observed_packets = 0;
		p.previously_observed_bytes = 0;
		__builtin_memset(&p.previously_observed_flags, 0, sizeof(struct tcpflagscount));
#ifdef USE_RING_BUFFER
		bpf_ringbuf_output(events_map, &p, sizeof(p), 0);
#else
		bpf_perf_event_output(skb, events_map, BPF_F_CURRENT_CPU, &p, sizeof(p));
#endif
		return;
	#elif DATA_AGGREGATION_LEVEL == DATA_AGGREGATION_LEVEL_HIGH
		if (report.report) {
			p.previously_observed_packets = report.previously_observed_packets;
			p.previously_observed_bytes = report.previously_observed_bytes;
			p.previously_observed_flags = report.previously_observed_flags;
#ifdef USE_RING_BUFFER
			bpf_ringbuf_output(events_map, &p, sizeof(p), 0);
#else
			bpf_perf_event_output(skb, events_map, BPF_F_CURRENT_CPU, &p, sizeof(p));
#endif
		}
	#endif
	#endif
	return;
}

#endif /* __PACKETPARSE_H__ */
