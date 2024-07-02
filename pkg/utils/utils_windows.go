// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"github.com/cilium/cilium/api/v1/flow"
)

func GetDropReasonDesc(dr DropReason) flow.DropReason {
	switch dr { //nolint:exhaustive // We are handling all the cases.
	case DropReason_Drop_INET_FinWait2:
		return flow.DropReason_UNKNOWN_CONNECTION_TRACKING_STATE
	default:
		return flow.DropReason_DROP_REASON_UNKNOWN
	}
}
