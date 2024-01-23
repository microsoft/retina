// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#include "vmlinux.h"

typedef enum
{
    IPTABLE_RULE_DROP = 0,
    IPTABLE_NAT_DROP,
    TCP_CONNECT_BASIC,
    TCP_ACCEPT_BASIC,
    TCP_CLOSE_BASIC,
    CONNTRACK_ADD_DROP,
    UNKNOWN_DROP,
} drop_reason_t;

#define NF_DROP 0

// https://elixir.bootlin.com/linux/v5.4.156/source/include/uapi/asm-generic/errno-base.h#L26
