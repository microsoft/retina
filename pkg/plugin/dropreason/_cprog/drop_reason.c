//go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_core_read.h"
#include "bpf_tracing.h"
#include "bpf_endian.h"
#include "drop_reason.h"
#include "dynamic.h"
#include "retina_filter.c"

char __license[] SEC("license") = "Dual MIT/GPL";

#define ETH_P_IP 0x0800
#define ETH_P_IPV6 0x86DD
#define ETH_P_8021Q 0x8100
#define ETH_P_ARP 0x0806
#define TASK_COMM_LEN 16
// TODO (Vamsi): Add top 100 dropped connections with LRU map

// Ref: https://elixir.bootlin.com/linux/latest/source/include/uapi/linux/if_packet.h#L26
#define PACKET_HOST 0     // Incomming packets
#define PACKET_OUTGOING 4 // Outgoing packets

struct metrics_map_key
{
    __u16 drop_type;
    __u32 return_val;
};
struct metrics_map_value
{
    __u64 count;
    __u64 bytes;
};

struct packet
{
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u32 skb_len;
    __u32 return_val;
    __u16 drop_type;
    __u8 proto;
    // in_filtermap defines if a given packet is of interest to us
    // and added to the filtermap. is this is set then dropreason
    // will send a perf event along with the usual aggregation in metricsmap
    bool in_filtermap;
    __u64 ts; // timestamp in nanoseconds
};
struct
{
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(max_entries, 16384);
} retina_dropreason_events SEC(".maps");

// Define const variables to avoid warnings.
const struct packet *unused __attribute__((unused));

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, __u32);
    __type(value, struct packet);
} retina_dropreason_natdrop_pids SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, __u32);
    __type(value, struct packet);
} retina_dropreason_drop_pids SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, __u32);
    __type(value, __u64);
} retina_dropreason_accept_pids SEC(".maps");

struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_HASH);
    __uint(max_entries, 512);
    __type(key, struct metrics_map_key);
    __type(value, struct metrics_map_value);
} retina_dropreason_metrics SEC(".maps");

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

void update_metrics_map(void *ctx, drop_reason_t drop_type, int ret_val, struct packet *p)
{
    struct metrics_map_value *entry, new_entry = {};
    struct metrics_map_key key;
    __builtin_memset(&key, 0, sizeof(p));
    key.drop_type = drop_type;
    key.return_val = ret_val;

    entry = bpf_map_lookup_elem(&retina_dropreason_metrics, &key);
    if (entry)
    {
        entry->count += 1;
        entry->bytes += p->skb_len;
    }
    else
    {
        new_entry.count = 1;
        new_entry.bytes = p->skb_len;
        bpf_map_update_elem(&retina_dropreason_metrics, &key, &new_entry, 0);
    }
// parse packet if advanced metrics are enabled
#ifdef ADVANCED_METRICS
#if ADVANCED_METRICS == 1
    if (p->in_filtermap)
    {
        p->drop_type = drop_type;
        p->return_val = ret_val;
        bpf_perf_event_output(ctx, &retina_dropreason_events, BPF_F_CURRENT_CPU, p, sizeof(struct packet));
    };
#endif
#endif
}

static void get_packet_from_skb(struct packet *p, struct sk_buff *skb)
{
    if (!skb)
    {
        return;
    }
    // TODO parse direction like in packetforward
    __u64 skb_len = 0;
    member_read(&skb_len, skb, len);
    p->skb_len = skb_len;

#ifdef ADVANCED_METRICS
#if ADVANCED_METRICS == 1
    char *head;
    __u16 nw_header, trans_header, eth_proto;

    member_read(&head, skb, head);
    member_read(&eth_proto, skb, protocol);
    member_read(&nw_header, skb, network_header);
    char *ip_header_address = head + nw_header;

    struct iphdr iphdr;
    member_read(&trans_header, skb, transport_header);
    bpf_probe_read(&iphdr, sizeof(iphdr), ip_header_address);

    // Check if the packet is of interest.
    #ifdef BYPASS_LOOKUP_IP_OF_INTEREST
	#if BYPASS_LOOKUP_IP_OF_INTEREST == 0
        if (!lookup(iphdr.saddr) && !lookup(iphdr.daddr))
            return;
    #endif
	#endif
    
    p->in_filtermap = true;
    p->src_ip = iphdr.saddr;
    p->dst_ip = iphdr.daddr;
    // get current timestamp in ns
    p->ts = bpf_ktime_get_boot_ns();

    if (iphdr.protocol == IPPROTO_TCP)
    {
        struct tcphdr tcphdr;
        char *tcphdraddr = head + trans_header;
        bpf_probe_read(&tcphdr, sizeof(tcphdr), tcphdraddr);
        p->src_port = bpf_htons(tcphdr.source);
        p->dst_port = bpf_htons(tcphdr.dest);
        p->proto = iphdr.protocol;
    }
    else if (iphdr.protocol == IPPROTO_UDP)
    {
        struct udphdr udphdr;
        char *udphdraddr = head + trans_header;
        bpf_probe_read(&udphdr, sizeof(udphdr), udphdraddr);
        p->src_port = bpf_htons(udphdr.source);
        p->dst_port = bpf_htons(udphdr.dest);
        p->proto = iphdr.protocol;
    }
#endif
#endif
}

static void get_packet_from_sock(struct packet *p, struct sock *sk)
{
    // extract 5 tuple from pid
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;

    BPF_CORE_READ_INTO(&saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
    BPF_CORE_READ_INTO(&sport, sk, __sk_common.skc_num);
    // Check if the packet is of interest.
    #ifdef BYPASS_LOOKUP_IP_OF_INTEREST
	#if BYPASS_LOOKUP_IP_OF_INTEREST == 0
        if (!lookup(saddr) && !lookup(daddr))
            return;
    #endif
	#endif

    // get current timestamp in ns
    p->ts = bpf_ktime_get_boot_ns();
    p->in_filtermap = true;
    p->src_ip = saddr;
    p->dst_ip = daddr;
    p->dst_port = bpf_ntohs(dport);
    p->src_port = bpf_ntohs(sport);
}

/*

This function will be saving the PID and length of skb it is working on.

*/

SEC("kprobe/nf_hook_slow")
int BPF_KPROBE(nf_hook_slow, struct sk_buff *skb, struct nf_hook_state *state)
{
    if (!skb)
        return 0;

    __u16 eth_proto;

    member_read(&eth_proto, skb, protocol);
    if (eth_proto != bpf_htons(ETH_P_IP))
        return 0;

    struct packet p;
    __builtin_memset(&p, 0, sizeof(p));
    get_packet_from_skb(&p, skb);

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    bpf_map_update_elem(&retina_dropreason_drop_pids, &pid, &p, BPF_ANY);
    return 0;
}

/*
This function will look PID and the length of SKB it is working on. Then it checks
the return value of the function and update the metrics map accordingly.

*/

SEC("kretprobe/nf_hook_slow")
int BPF_KRETPROBE(nf_hook_slow_ret, int retVal)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    if (retVal >= 0)
    {
        bpf_map_delete_elem(&retina_dropreason_drop_pids, &pid);
        return 0;
    }

    struct packet *p = bpf_map_lookup_elem(&retina_dropreason_drop_pids, &pid);
    if (!p)
    {
        return 0;
    }

    bpf_map_delete_elem(&retina_dropreason_drop_pids, &pid);

    update_metrics_map(ctx, IPTABLE_RULE_DROP, 0, p);
    return 0;
}

/*
This function checks the return value of tcp_v4_connect and
 update the metrics map accordingly.

 tcp_v4_connect does not have any lenth attached to it.
*/

SEC("kretprobe/tcp_v4_connect")
int BPF_KRETPROBE(tcp_v4_connect_ret, int retVal)
{
    if (retVal == 0)
    {
        return 0;
    }

    struct packet p;
    __builtin_memset(&p, 0, sizeof(p));

    update_metrics_map(ctx, TCP_CONNECT_BASIC, retVal, &p);
    return 0;
}

SEC("kprobe/inet_csk_accept")
int BPF_KPROBE(inet_csk_accept, struct sock *sk, int flags, int *err, bool kern)
{
    /*
    This function will save the reference value to error.
    in kretprobe we look at the value we got back in that reference
    */
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 err_ptr = (__u64)err;
    bpf_map_update_elem(&retina_dropreason_accept_pids, &pid, &err_ptr, BPF_ANY);
    return 0;
}

SEC("kretprobe/inet_csk_accept")
int BPF_KRETPROBE(inet_csk_accept_ret, struct sock *sk)
{
    /*
        //https://elixir.bootlin.com/linux/v5.4.156/source/net/ipv4/af_inet.c#L734
        //if accept returns empty sock, then this function errored.

        //errors:
        //https://elixir.bootlin.com/linux/v5.4.156/source/include/uapi/asm-generic/errno-base.h#L26
        //TODO: use tracepoint

    */
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 *err_ptr = bpf_map_lookup_elem(&retina_dropreason_accept_pids, &pid);
    bpf_map_delete_elem(&retina_dropreason_accept_pids, &pid);

    if (!err_ptr)
        return 0;

    if (sk != NULL)
        return 0;

    int err = (int)*err_ptr;
    if (err >= 0)
        return 0;

    struct packet p;
    __builtin_memset(&p, 0, sizeof(p));

    p.in_filtermap = false;
    p.skb_len = 0;

#ifdef ADVANCED_METRICS
#if ADVANCED_METRICS == 1
    get_packet_from_sock(&p, sk);
#endif
#endif

    update_metrics_map(ctx, TCP_ACCEPT_BASIC, err, &p);
    return 0;
}

SEC("kprobe/nf_nat_inet_fn")
int BPF_KPROBE(nf_nat_inet_fn, void *priv, struct sk_buff *skb, const struct nf_hook_state *state)
{
    if (!skb)
        return 0;

    __u16 eth_proto;

    member_read(&eth_proto, skb, protocol);
    if (eth_proto != bpf_htons(ETH_P_IP))
        return 0;

    struct packet p;
    __builtin_memset(&p, 0, sizeof(p));
    get_packet_from_skb(&p, skb);

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    bpf_map_update_elem(&retina_dropreason_natdrop_pids, &pid, &p, BPF_ANY);
    return 0;
}

SEC("kretprobe/nf_nat_inet_fn")
int BPF_KRETPROBE(nf_nat_inet_fn_ret, int retVal)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    if (retVal != NF_DROP)
    {
        bpf_map_delete_elem(&retina_dropreason_natdrop_pids, &pid);
        return 0;
    }

    struct packet *p = bpf_map_lookup_elem(&retina_dropreason_natdrop_pids, &pid);
    if (!p)
    {
        return 0;
    }

    bpf_map_delete_elem(&retina_dropreason_natdrop_pids, &pid);

    update_metrics_map(ctx, IPTABLE_NAT_DROP, 0, p);
    return 0;
}

SEC("kprobe/__nf_conntrack_confirm")
int BPF_KPROBE(nf_conntrack_confirm, struct sk_buff *skb)
{
    if (!skb)
        return 0;

    __u16 eth_proto;

    member_read(&eth_proto, skb, protocol);
    if (eth_proto != bpf_htons(ETH_P_IP))
        return 0;

    struct packet p;
    __builtin_memset(&p, 0, sizeof(p));
    get_packet_from_skb(&p, skb);

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    bpf_map_update_elem(&retina_dropreason_natdrop_pids, &pid, &p, BPF_ANY);
    return 0;
}

SEC("kretprobe/__nf_conntrack_confirm")
int BPF_KRETPROBE(nf_conntrack_confirm_ret, int retVal)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    
    if (retVal != NF_DROP)
    {
        bpf_map_delete_elem(&retina_dropreason_natdrop_pids, &pid);
        return 0;
    }
    
    struct packet *p = bpf_map_lookup_elem(&retina_dropreason_natdrop_pids, &pid);
    if (!p)
    {
        return 0;
    }

    bpf_map_delete_elem(&retina_dropreason_natdrop_pids, &pid);

    update_metrics_map(ctx, CONNTRACK_ADD_DROP, retVal, p);
    return 0;
}
