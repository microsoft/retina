//go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_endian.h"
#include "packetparser.h"
#include "conntrack.c"
#include "conntrack.h"
#include "retina_filter.c"
#include "dynamic.h"

char __license[] SEC("license") = "Dual MIT/GPL";

struct
{
#ifdef USE_RING_BUFFER
	__uint(type, BPF_MAP_TYPE_RINGBUF);
#ifndef RING_BUFFER_SIZE
#define RING_BUFFER_SIZE (8 * 1024 * 1024)
#endif
	__uint(max_entries, RING_BUFFER_SIZE);
#else
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(max_entries, 16384);
#endif
} retina_packetparser_events SEC(".maps");

// Define const variables to avoid warnings.
const struct packet *unused __attribute__((unused));

#include "packetparse.h"

SEC("classifier_endpoint_ingress")
int endpoint_ingress_filter(struct __sk_buff *skb)
{
	// This is attached to the interface on the host side.
	// So ingress on host is egress on endpoint and vice versa.
	parse(skb, OBSERVATION_POINT_FROM_ENDPOINT, &retina_packetparser_events);
	// Always return TC_ACT_UNSPEC to allow packet to pass to the next BPF program.
	return TC_ACT_UNSPEC;
}

SEC("classifier_endpoint_egress")
int endpoint_egress_filter(struct __sk_buff *skb)
{
	// This is attached to the interface on the host side.
	// So egress on host is ingress on endpoint and vice versa.
	parse(skb, OBSERVATION_POINT_TO_ENDPOINT, &retina_packetparser_events);
	// Always return TC_ACT_UNSPEC to allow packet to pass to the next BPF program.
	return TC_ACT_UNSPEC;
}

SEC("classifier_host_ingress")
int host_ingress_filter(struct __sk_buff *skb)
{
	parse(skb, OBSERVATION_POINT_FROM_NETWORK, &retina_packetparser_events);
	// Always return TC_ACT_UNSPEC to allow packet to pass to the next BPF program.
	return TC_ACT_UNSPEC;
}

SEC("classifier_host_egress")
int host_egress_filter(struct __sk_buff *skb)
{
	parse(skb, OBSERVATION_POINT_TO_NETWORK, &retina_packetparser_events);
	// Always return TC_ACT_UNSPEC to allow packet to pass to the next BPF program.
	return TC_ACT_UNSPEC;
}
