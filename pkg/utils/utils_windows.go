// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"fmt"

	"github.com/cilium/cilium/api/v1/flow"
)

func GetDropReasonDesc(dr DropReason) flow.DropReason {
	fmt.Printf("getting drop reason description for %v\n", dr)
	switch dr {
	case DropReason_Drop_INET_FinWait2:
		fmt.Printf("setting drop as  %v\n", flow.DropReason_TTL_EXCEEDED)
		return flow.DropReason_TTL_EXCEEDED
	default:
		return flow.DropReason_DROP_REASON_UNKNOWN
	}
}
