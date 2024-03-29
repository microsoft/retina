// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

#define ETH_P_IP	0x0800
// The maximum length of the TCP options field.
#define MAX_TCP_OPTIONS_LEN 40

typedef enum
{
    FROM_ENDPOINT = 0,
    TO_ENDPOINT,
    FROM_NETWORK,
    TO_NETWORK,
} direction;
