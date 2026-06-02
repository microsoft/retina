// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"github.com/cilium/cilium/api/v1/flow"
)

func GetDropReasonDesc(dr DropReason) flow.DropReason {
	// Keep mapping aligned with Linux where drop reasons overlap.
	switch dr { //nolint:exhaustive // We are handling all the cases.
	case DropReason_IPTABLE_RULE_DROP:
		return flow.DropReason_POLICY_DENIED
	case DropReason_IPTABLE_NAT_DROP:
		return flow.DropReason_SNAT_NO_MAP_FOUND
	case DropReason_CONNTRACK_ADD_DROP:
		return flow.DropReason_UNKNOWN_CONNECTION_TRACKING_STATE
	default:
		return flow.DropReason_DROP_REASON_UNKNOWN
	}
}
