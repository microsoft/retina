// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#define ETH_P_IP	0x0800
// The maximum length of the TCP options field.
#define MAX_TCP_OPTIONS_LEN 40
// tc-bpf return code to execute the next tc-bpf program.
#define TC_ACT_UNSPEC   (-1)

typedef enum
{
    DATA_AGGREGATION_LEVEL_LOW = 0,
    DATA_AGGREGATION_LEVEL_HIGH,
} data_aggregation_level;
