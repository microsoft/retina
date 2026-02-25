//go:build ignore

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// TCX (TC eXpress) variant of the packetparser BPF program.
// Uses tcx/ section types for attachment via the TCX mechanism (kernel 6.6+).
// Shares the parse() logic with the traditional TC packetparser via packetparse.h.

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
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(max_entries, 16384);
} retina_packetparser_tcx_events SEC(".maps");

const struct packet *unused __attribute__((unused));

#include "packetparse.h"

SEC("tcx/ingress")
int endpoint_ingress_filter(struct __sk_buff *skb)
{
	parse(skb, OBSERVATION_POINT_FROM_ENDPOINT, &retina_packetparser_tcx_events);
	return TCX_NEXT;
}

SEC("tcx/egress")
int endpoint_egress_filter(struct __sk_buff *skb)
{
	parse(skb, OBSERVATION_POINT_TO_ENDPOINT, &retina_packetparser_tcx_events);
	return TCX_NEXT;
}

SEC("tcx/ingress")
int host_ingress_filter(struct __sk_buff *skb)
{
	parse(skb, OBSERVATION_POINT_FROM_NETWORK, &retina_packetparser_tcx_events);
	return TCX_NEXT;
}

SEC("tcx/egress")
int host_egress_filter(struct __sk_buff *skb)
{
	parse(skb, OBSERVATION_POINT_TO_NETWORK, &retina_packetparser_tcx_events);
	return TCX_NEXT;
}
