// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// Based on: https://github.com/iovisor/bcc/blob/master/libbpf-tools/tcpconnect.c
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_core_read.h"
#include "bpf_tracing.h"
#include "bpf_endian.h"
// #include <arpa/inet.h>

char __license[] SEC("license") = "Dual MIT/GPL";

#define CONNECT 1
#define ACCEPT 2
#define CLOSE 3
#define TASK_COMM_LEN 16

struct tcpv4event
{
	__u32 pid;
	__u64 ts;
	__u8 comm[TASK_COMM_LEN];
	__u32 saddr;
	__u32 daddr;
	__u16 dport;
	__u16 sport;
	__u64 sent_bytes;
	__s32 recv_bytes;
	__u16 operation;
	__u8 padding[1];
};

struct
{
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 16384);
	__type(key, u32);
	__type(value, struct sock *);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} sockets SEC(".maps");

struct
{
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(max_entries, 16384);
} tcpv4connect SEC(".maps");

struct
{
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(max_entries, 16384);
} tcpv4accept SEC(".maps");

struct
{
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(max_entries, 16384);
} tcpv4close SEC(".maps");

// struct
// {
// 	__uint(type, BPF_MAP_TYPE_RINGBUF);
// } events_ring SEC(".maps");

const struct tcpv4event *unusedv4 __attribute__((unused));

// static __always_inline int
// enter_tcp_connect(struct pt_regs *ctx, struct sock *sk)
SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(tcp_v4_connect, struct sock *sk)
{
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	__u32 pid = pid_tgid >> 32;
	bpf_printk("enter: setting sk for tid..: %ld\n", pid);
	bpf_map_update_elem(&sockets, &pid, &sk, BPF_ANY);
	return 0;
}

// static __always_inline int
// exit_tcp_connect(struct pt_regs *ctx, int ret)

SEC("kretprobe/tcp_v4_connect")
int BPF_KRETPROBE(tcp_v4_connect_ret, int ret)
{
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	__u32 pid = pid_tgid >> 32;
	struct sock **skpp;
	struct sock *sk;

	__u32 saddr;
	__u32 daddr;
	__u16 dport;
	__u16 sport;

	skpp = bpf_map_lookup_elem(&sockets, &pid);
	if (!skpp)
	{
		bpf_printk("exit: no pointer for tid, returning..: %ld", pid);
		return 0;
	}

	sk = *skpp;

	BPF_CORE_READ_INTO(&saddr, sk, __sk_common.skc_rcv_saddr);
	BPF_CORE_READ_INTO(&daddr, sk, __sk_common.skc_daddr);
	BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
	BPF_CORE_READ_INTO(&sport, sk, __sk_common.skc_num);

	__u64 ts = bpf_ktime_get_ns();
	struct tcpv4event et;
	__builtin_memset(&et, 0, sizeof(et));
	et.pid = pid;
	et.ts = ts;
	et.saddr = saddr;
	et.daddr = daddr;
	et.dport = bpf_ntohs(dport);
	et.sport = sport;
	et.operation = CONNECT;

	if (saddr != daddr)
	{
		bpf_get_current_comm(et.comm, sizeof(et.comm));
		bpf_perf_event_output(ctx, &tcpv4connect, BPF_F_CURRENT_CPU, &et, sizeof(et));
	}

	bpf_map_delete_elem(&sockets, &pid);
	return 0;
}

SEC("kretprobe/inet_csk_accept")
int BPF_KRETPROBE(inet_csk_accept_ret, struct sock *sk)
{
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	__u32 pid = pid_tgid >> 32;

	if (sk == NULL)
		return 0;

	__u32 saddr;
	__u32 daddr;
	__u16 dport;
	__u16 sport;

	BPF_CORE_READ_INTO(&saddr, sk, __sk_common.skc_rcv_saddr);
	BPF_CORE_READ_INTO(&daddr, sk, __sk_common.skc_daddr);
	BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
	BPF_CORE_READ_INTO(&sport, sk, __sk_common.skc_num);

	__u64 ts = bpf_ktime_get_ns();
	struct tcpv4event et;
	__builtin_memset(&et, 0, sizeof(et));
	et.pid = pid;
	et.ts = ts;
	et.saddr = saddr;
	et.daddr = daddr;
	et.dport = bpf_ntohs(dport);
	et.sport = sport;
	et.operation = ACCEPT;

	if (saddr != daddr)
	{
		bpf_get_current_comm(et.comm, sizeof(et.comm));
		bpf_perf_event_output(ctx, &tcpv4accept, BPF_F_CURRENT_CPU, &et, sizeof(et));
	}

	return 0;
}

SEC("kprobe/tcp_close")
int BPF_KPROBE(tcp_close, struct sock *sk, long timeout)
{
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	__u32 pid = pid_tgid >> 32;

	if (sk == NULL)
		return 0;

	u16 oldstate; //= sk->sk_state;
	BPF_CORE_READ_INTO(&oldstate, sk, __sk_common.skc_state);
	bpf_printk("close old state:%d\n", oldstate);

	// Don't generate close events for connections that were never
	// established in the first place.
	if (oldstate == TCP_SYN_SENT ||
		oldstate == TCP_SYN_RECV ||
		oldstate == TCP_NEW_SYN_RECV)
		return 0;

	__u32 saddr;
	__u32 daddr;
	__u16 dport;
	__u16 sport;

	BPF_CORE_READ_INTO(&saddr, sk, __sk_common.skc_rcv_saddr);
	BPF_CORE_READ_INTO(&daddr, sk, __sk_common.skc_daddr);
	BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
	BPF_CORE_READ_INTO(&sport, sk, __sk_common.skc_num);

	__u64 ts = bpf_ktime_get_ns();
	struct tcpv4event et;
	__builtin_memset(&et, 0, sizeof(et));
	et.pid = pid;
	et.ts = ts;
	et.saddr = saddr;
	et.daddr = daddr;
	et.dport = bpf_ntohs(dport);
	et.sport = sport;
	et.operation = CLOSE;

	if (saddr != daddr)
	{
		bpf_get_current_comm(et.comm, sizeof(et.comm));
		bpf_perf_event_output(ctx, &tcpv4close, BPF_F_CURRENT_CPU, &et, sizeof(et));
	}

	return 0;
}
