// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_core_read.h"
#include "bpf_tracing.h"
#include "bpf_endian.h"
// #include <arpa/inet.h>

char __license[] SEC("license") = "Dual MIT/GPL";

#define ETH_P_IP 0x0800
#define ETH_P_IPV6 0x86DD
#define ETH_P_8021Q 0x8100
#define ETH_P_ARP 0x0806
#define TASK_COMM_LEN 16

struct iptuple
{
	__u16 proto;
	__u32 saddr;
	__u32 daddr;
	__u16 dport;
	__u16 sport;
	__u32 hook;
	char devname[32];
	__u16 padding;
};

struct verdict
{
	struct iptuple flow;
	__u64 ts;
	__u32 pid;
	unsigned char comm[TASK_COMM_LEN];
	int status;
};

struct
{
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 16384);
	__type(key, u32);
	__type(value, struct iptuple);
} ipflows SEC(".maps");

struct
{
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(max_entries, 16384);
} verdicts SEC(".maps");

const struct verdict *unused __attribute__((unused));

#define MAC_HEADER_SIZE 14;
#define member_address(source_struct, source_member)                                                 \
	({                                                                                               \
		void *__ret;                                                                                 \
		__ret = (void *)(((char *)source_struct) + offsetof(typeof(*source_struct), source_member)); \
		__ret;                                                                                       \
	})
#define member_read(destination, source_struct, source_member) \
	do                                                         \
	{                                                          \
		bpf_probe_read(                                        \
			destination,                                       \
			sizeof(source_struct->source_member),              \
			member_address(source_struct, source_member));     \
	} while (0)

SEC("kprobe/nf_hook_slow")
int BPF_KPROBE(nf_hook_slow, struct sk_buff *skb, struct nf_hook_state *state)
{
	char *head;
	__u16 mac_header, nw_header, tcp_header, eth_proto;

	if (skb)
	{
		member_read(&head, skb, head);
		member_read(&mac_header, skb, mac_header);
		member_read(&nw_header, skb, network_header);
		member_read(&eth_proto, skb, protocol);
		char *ip_header_address = head + nw_header;
		if (eth_proto == bpf_htons(ETH_P_IP))
		{
			struct iphdr iphdr;
			__u16 sport, dport;

			member_read(&tcp_header, skb, transport_header);
			struct iptuple ipt;
			__builtin_memset(&ipt, 0, sizeof(ipt));
			// event.ip_version = 4;
			bpf_probe_read(&iphdr, sizeof(iphdr), ip_header_address);
			__u8 l4proto = iphdr.protocol;
			// Discard non UDP traffic
			if (l4proto != IPPROTO_TCP)
			{
				return 0;
			}
			// event.addr.v4.saddr = iphdr.saddr;
			// event.addr.v4.daddr = iphdr.daddr;
			// struct tcphdr *tcphdr = (struct tcphdr *)(&ip_header_address[offset]);
			struct tcphdr tcphdr;
			char *tcphdraddr = head + tcp_header;
			bpf_probe_read(&tcphdr, sizeof(tcphdr), tcphdraddr);
			sport = bpf_htons(tcphdr.source);
			dport = bpf_htons(tcphdr.dest);

			ipt.proto = iphdr.protocol;
			ipt.saddr = iphdr.saddr;
			ipt.daddr = iphdr.daddr;
			ipt.sport = sport;
			ipt.dport = dport;
			member_read(&ipt.hook, state, hook);
			// bpf_printk("devname:%s", ipt.devname);
			// bpf_printk("kprobe:sport: %d dport:%d hook:%d\n", ipt.sport, ipt.dport, ipt.hook);
			__u64 pid_tgid = bpf_get_current_pid_tgid();
			__u32 pid = pid_tgid >> 32;
			// if (sport == 4567 || dport ==4567)
			// {
			bpf_map_update_elem(&ipflows, &pid, &ipt, BPF_ANY);
			// }
		}
	}
	return 0;
}

SEC("kretprobe/nf_hook_slow")
int BPF_KRETPROBE(nf_hook_slow_ret, int verdict)
{
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	__u32 pid = pid_tgid >> 32;
	struct iptuple *ipflow = bpf_map_lookup_elem(&ipflows, &pid);
	if (!ipflow)
	{
		return 0;
	}

	if (verdict >= 0)
	{
		return 0;
	}

	__u64 ts = bpf_ktime_get_ns();
	struct verdict v = {
		.ts = ts,
		.flow = *ipflow,
		.status = verdict,
		.pid = pid};

	bpf_get_current_comm(v.comm, sizeof(v.comm));

	bpf_perf_event_output(ctx, &verdicts, BPF_F_CURRENT_CPU, &v, sizeof(v));
	bpf_map_delete_elem(&ipflows, &pid);
	return 0;
}
