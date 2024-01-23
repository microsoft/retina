// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_core_read.h"
#include "bpf_tracing.h"
#include "bpf_endian.h"

char __license[] SEC("license") = "Dual MIT/GPL";

#define TASK_COMM_LEN 16

struct tcpsendEvent
{
	__u32 pid;
	__u64 ts;
	__u8 comm[TASK_COMM_LEN];
	__u32 saddr;
	__u32 daddr;
	__u16 dport;
	__u16 sport;
	__u16 l4proto;
	__u64 sent_bytes;
	__u16 operation;
	__u8 padding[1];
};

// mapkey is struct to define a 5-tuple as a key for mapevent
struct mapkey
{
	__u32 saddr;
	__u32 daddr;
	__u16 sport;
	__u16 dport;
	__u16 l4proto;
};
struct mapkey *unused_k __attribute__((unused));

// mapevent adds all unique mapkeys to the hashtable
// value is the amount of bytes sent
struct
{
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 4096);
	__type(key, struct mapkey);
	__type(value, __u64);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} mapevent SEC(".maps");

const struct tcpsendEvent *unusedv4 __attribute__((unused));

SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(tcp_sendmsg, struct sock *sk,
			   struct msghdr *msg, size_t size)
{
	__u32 saddr;
	__u32 daddr;
	__u16 dport;
	__u16 sport;
	__u8 l4proto;
	BPF_CORE_READ_INTO(&saddr, sk, __sk_common.skc_rcv_saddr);
	BPF_CORE_READ_INTO(&daddr, sk, __sk_common.skc_daddr);
	BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
	BPF_CORE_READ_INTO(&sport, sk, __sk_common.skc_num);
	BPF_CORE_READ_INTO(&l4proto, sk, sk_protocol);

	struct mapkey k;
	__builtin_memset(&k, 0, sizeof(k));
	k.saddr = saddr;
	k.daddr = daddr;
	k.sport = sport;
	k.dport = bpf_ntohs(dport);
	k.l4proto = l4proto;

	__u64 initval = size, *val;
	val = bpf_map_lookup_elem(&mapevent, &k);
	if (!val)
	{
		int err = bpf_map_update_elem(&mapevent, &k, &initval, BPF_ANY);
		if (err == -1)
		{
			bpf_printk("Could not update map: %d", err);
			return 0;
		}
	}
	else
	{
		// adding send bytes
		__sync_fetch_and_add(val, size);
	}
	return 0;
}
