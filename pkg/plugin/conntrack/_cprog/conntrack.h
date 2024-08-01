// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#pragma once

#include "vmlinux.h"
#include "bpf_helpers.h"


#define NSEC_PER_SEC	(1000ULL * 1000ULL * 1000UL)
#define NSEC_PER_MSEC	(1000ULL * 1000ULL)
#define NSEC_PER_USEC	(1000UL)

/* Monotonic clock, scalar format. */
#define bpf_ktime_get_sec()	\
	({ __u64 __x = bpf_ktime_get_ns() / NSEC_PER_SEC; __x; })
#define bpf_ktime_get_msec()	\
	({ __u64 __x = bpf_ktime_get_ns() / NSEC_PER_MSEC; __x; })
#define bpf_ktime_get_usec()	\
	({ __u64 __x = bpf_ktime_get_ns() / NSEC_PER_USEC; __x; })
#define bpf_ktime_get_nsec()	\
	({ __u64 __x = bpf_ktime_get_ns(); __x; })

# define bpf_mono_now()		bpf_ktime_get_sec()

#define CT_CONNECTION_LIFETIME_TCP  360 // 6 minutes
#define CT_CONNECTION_LIFETIME_NONTCP	60 // 1 minute
#define CT_SYN_TIMEOUT                  60 // 1 minute
#define CT_REPORT_INTERVAL              5 // 5 seconds

enum tcp_flags {
    TCP_FIN = 0x01,
    TCP_SYN = 0x02,
    TCP_RST = 0x04,
    TCP_PSH = 0x08,
    TCP_ACK = 0x10,
    TCP_URG = 0x20,
    TCP_ECE = 0x40,
    TCP_CWR = 0x80,
    TCP_NS  = 0x100
};
