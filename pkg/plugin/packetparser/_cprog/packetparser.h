// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#define ETH_P_IP	0x0800
// The maximum length of the TCP options field.
#define MAX_TCP_OPTIONS_LEN 40
// tc-bpf return code to execute the next tc-bpf program.
#define TC_ACT_UNSPEC   (-1)
// tcx return code to pass to the next program in the chain (kernel 6.6+).
#define TCX_NEXT        (-1)

#define DATA_AGGREGATION_LEVEL_LOW 0
#define DATA_AGGREGATION_LEVEL_HIGH 1
