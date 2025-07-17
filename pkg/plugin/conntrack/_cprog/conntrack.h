// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#pragma once

#include "vmlinux.h"
#include "bpf_helpers.h"

// Helper functions to get the current time
// Ref: https://github.com/cilium/cilium/blob/6186d579ed60f334c7a4daaf81060797b02cc6bd/bpf/lib/time.h
#define NSEC_PER_SEC	(1000ULL * 1000ULL * 1000UL)
#define bpf_ktime_get_sec()	\
	({ __u64 __x = bpf_ktime_get_boot_ns() / NSEC_PER_SEC; __x; })
# define bpf_mono_now()		bpf_ktime_get_sec()

#define UINT32_MAX 4294967295U

// Time units in seconds

// Define how long a TCP connection should be kept in the table
#define CT_CONNECTION_LIFETIME_TCP 360
// Define how long a TCP connection should be kept in the TIME_WAIT state
#define CT_TIME_WAIT_TIMEOUT_TCP 30
// Define how long a non-TCP connection should be kept in the table
#define CT_CONNECTION_LIFETIME_NONTCP 60
// Define how long a TCP connection should be kept alive after receiving the first SYN
#define CT_SYN_TIMEOUT 60
// Define the interval at which a packet should be sent to the userspace
#define CT_REPORT_INTERVAL 30
// Define the maximum number of connections that can be stored in the conntrack table
#define CT_MAP_SIZE 262144

#define TCP_FIN 0x01
#define TCP_SYN 0x02
#define TCP_RST 0x04
#define TCP_PSH 0x08
#define TCP_ACK 0x10
#define TCP_URG 0x20
#define TCP_ECE 0x40
#define TCP_CWR 0x80
#define TCP_NS 0x100

#define CT_PACKET_DIR_TX 0x00
#define CT_PACKET_DIR_RX 0x01

#define TRAFFIC_DIRECTION_UNKNOWN 0x00
#define TRAFFIC_DIRECTION_INGRESS 0x01
#define TRAFFIC_DIRECTION_EGRESS 0x02

#define OBSERVATION_POINT_FROM_ENDPOINT 0x00
#define OBSERVATION_POINT_TO_ENDPOINT 0x01
#define OBSERVATION_POINT_FROM_NETWORK 0x02
#define OBSERVATION_POINT_TO_NETWORK 0x03
