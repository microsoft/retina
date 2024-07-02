// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import "github.com/cilium/cilium/api/v1/flow"

// Alert: this ordering should match the drop_reason_t enum ordering
// in dropreason.h of DropReason plugin
const (
	IPTABLE_RULE_DROP DropReasonType = iota
	IPTABLE_NAT_DROP
	TCP_CONNECT_BASIC
	TCP_ACCEPT_BASIC
	TCP_CLOSE_BASIC
	CONNTRACK_ADD_DROP
	UNKNOWN_DROP
)

func GetDropType(value uint32) DropReasonType {
	switch value {
	case 0:
		return IPTABLE_RULE_DROP
	case 1:
		return IPTABLE_NAT_DROP
	case 2:
		return TCP_CONNECT_BASIC
	case 3:
		return TCP_ACCEPT_BASIC
	case 4:
		return TCP_CLOSE_BASIC
	case 5:
		return CONNTRACK_ADD_DROP
	default:
		return UNKNOWN_DROP
	}
}

func GetDropTypeFlowDropReason(dr flow.DropReason) string {
	return GetDropType(uint32(dr)).String()
}

func (d DropReasonType) String() string {
	switch d {
	case IPTABLE_RULE_DROP:
		return "IPTABLE_RULE_DROP"
	case IPTABLE_NAT_DROP:
		return "IPTABLE_NAT_DROP"
	case TCP_CONNECT_BASIC:
		return "TCP_CONNECT_BASIC"
	case TCP_ACCEPT_BASIC:
		return "TCP_ACCEPT_BASIC"
	case TCP_CLOSE_BASIC:
		return "TCP_CLOSE_BASIC"
	case CONNTRACK_ADD_DROP:
		return "CONNTRACK_ADD_DROP"
	case UNKNOWN_DROP:
		return "UNKNOWN_DROP"
	default:
		return "UNKNOWN_DROP"
	}
}
