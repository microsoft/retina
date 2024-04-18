// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"net"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/monitor/api"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Additional Verdicts to be used for flow objects
const (
	Verdict_RETRANSMISSION flow.Verdict = 15
	Verdict_DNS            flow.Verdict = 16
	TypeUrl                string       = "retina.sh"
)

// ToFlow returns a flow.Flow object.
// This sets up a L3/L4 flow object.
// sourceIP, destIP are IPv4 addresses.
// sourcePort, destPort are TCP/UDP ports.
// proto is the protocol number. Ref: https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml .
// observationPoint is the observation point+direction of the flow. 0 is from n/w stack to container, 1 is from container to stack,
// 2 is from host to network and 3 is from network to host.
// ts is the timestamp in nanoseconds.
func ToFlow(
	ts int64,
	sourceIP, destIP net.IP,
	sourcePort, destPort uint32,
	proto uint8,
	observationPoint uint32,
	verdict flow.Verdict,
) *flow.Flow { //nolint:typecheck
	var (
		l4           *flow.Layer4
		checkpoint   flow.TraceObservationPoint
		direction    flow.TrafficDirection
		subeventtype int
	)

	l := log.Logger().Named("ToFlow")

	switch proto {
	case 6:
		l4 = &flow.Layer4{
			Protocol: &flow.Layer4_TCP{
				TCP: &flow.TCP{
					SourcePort:      sourcePort,
					DestinationPort: destPort,
				},
			},
		}
	case 17:
		l4 = &flow.Layer4{
			Protocol: &flow.Layer4_UDP{
				UDP: &flow.UDP{
					SourcePort:      sourcePort,
					DestinationPort: destPort,
				},
			},
		}
	}

	// We are attaching the filters to the veth interface on the host side.
	// So for HOST -> CONTAINER, egress of host veth is ingress of container.
	// Hence, we need to swap the direction.
	switch observationPoint {
	case uint32(0):
		checkpoint = flow.TraceObservationPoint_TO_STACK
		direction = flow.TrafficDirection_EGRESS
		subeventtype = int(api.TraceToStack)
	case uint32(1):
		checkpoint = flow.TraceObservationPoint_TO_ENDPOINT
		direction = flow.TrafficDirection_INGRESS
		subeventtype = int(api.TraceToLxc)
	case uint32(2):
		checkpoint = flow.TraceObservationPoint_FROM_NETWORK
		direction = flow.TrafficDirection_INGRESS
		subeventtype = int(api.TraceFromNetwork)
	case uint32(3):
		checkpoint = flow.TraceObservationPoint_TO_NETWORK
		direction = flow.TrafficDirection_EGRESS
		subeventtype = int(api.TraceToNetwork)
	default:
		checkpoint = flow.TraceObservationPoint_UNKNOWN_POINT
		direction = flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN
	}

	if verdict == 0 {
		verdict = flow.Verdict_FORWARDED
	}

	ext, _ := anypb.New(&RetinaMetadata{}) //nolint:typecheck

	f := &flow.Flow{
		Type: flow.FlowType_L3_L4,
		EventType: &flow.CiliumEventType{
			Type:    int32(api.MessageTypeTrace),
			SubType: int32(subeventtype),
		},
		IP: &flow.IP{
			Source:      sourceIP.String(),
			Destination: destIP.String(),
			// We only support IPv4 for now.
			IpVersion: flow.IPVersion_IPv4,
		},
		L4:                    l4,
		TraceObservationPoint: checkpoint,
		TrafficDirection:      direction,
		Verdict:               verdict,
		Extensions:            ext,
		IsReply:               &wrapperspb.BoolValue{Value: false}, // Setting false by default as we don't have a better way to determine flow direction.
	}
	if t, err := decodeTime(ts); err == nil {
		f.Time = t
	} else {
		l.Warn("Failed to get current time", zap.Error(err))
	}
	return f
}

func AddTcpFlags(f *flow.Flow, syn, ack, fin, rst, psh, urg uint16) {
	if f.L4.GetTCP() == nil {
		return
	}

	f.L4.GetTCP().Flags = &flow.TCPFlags{
		SYN: syn == uint16(1),
		ACK: ack == uint16(1),
		FIN: fin == uint16(1),
		RST: rst == uint16(1),
		PSH: psh == uint16(1),
		URG: urg == uint16(1),
	}
}

// Add TSval/TSecr to the flow as TCP ID.
// The TSval/TSecr works as ID for the flow.
// We will use this ID to calculate latency.
func AddTcpID(f *flow.Flow, id uint64) {
	if f.L4 == nil || f.L4.GetTCP() == nil {
		return
	}
	k := &RetinaMetadata{}      //nolint:typecheck
	f.Extensions.UnmarshalTo(k) //nolint:errcheck
	k.TcpId = id
	f.Extensions, _ = anypb.New(k) //nolint:errcheck
}

func GetTcpID(f *flow.Flow) uint64 {
	if f.L4 == nil || f.L4.GetTCP() == nil {
		return 0
	}
	k := &RetinaMetadata{}      //nolint:typecheck
	f.Extensions.UnmarshalTo(k) //nolint:errcheck
	return k.TcpId
}

func AddDnsInfo(f *flow.Flow, qType string, rCode uint32, query string, qTypes []string, numAnswers int, ips []string) {
	if f == nil {
		return
	}
	// Set type to L7.
	f.Type = flow.FlowType_L7
	// Reset Eventtype if already set at L3/4 level.
	f.EventType = &flow.CiliumEventType{
		Type: int32(api.MessageTypeNames[api.MessageTypeNameL7]),
	}
	l7 := flow.Layer7_Dns{
		Dns: &flow.DNS{
			Rcode:  rCode,
			Query:  query,
			Qtypes: qTypes,
			Ips:    ips,
		},
	}
	f.L7 = &flow.Layer7{
		Record: &l7,
	}
	k := &RetinaMetadata{}      //nolint:typecheck
	f.Extensions.UnmarshalTo(k) //nolint:errcheck
	switch qType {
	case "Q":
		k.DnsType = DNSType_QUERY
		f.L7.Type = flow.L7FlowType_REQUEST
	case "R":
		k.DnsType = DNSType_RESPONSE
		f.L7.Type = flow.L7FlowType_RESPONSE
		f.IsReply = &wrapperspb.BoolValue{Value: true} // we can definitely say that this is a reply
	default:
		k.DnsType = DNSType_UNKNOWN
		f.L7.Type = flow.L7FlowType_UNKNOWN_L7_TYPE
	}
	k.NumResponses = uint32(numAnswers)
	f.Extensions, _ = anypb.New(k)
}

func GetDns(f *flow.Flow) (*flow.DNS, DNSType, uint32) {
	if f == nil || f.L7 == nil || f.L7.GetDns() == nil {
		return nil, DNSType_UNKNOWN, 0
	}
	dns := f.L7.GetDns()
	if f.Extensions == nil {
		return dns, DNSType_UNKNOWN, 0
	}
	k := &RetinaMetadata{}      //nolint:typecheck
	f.Extensions.UnmarshalTo(k) //nolint:errcheck

	return dns, k.DnsType, k.NumResponses
}

// DNS Return code to string.
func DnsRcodeToString(f *flow.Flow) string {
	if f == nil || f.L7 == nil || f.L7.GetDns() == nil {
		return ""
	}
	switch f.L7.GetDns().Rcode {
	case 0:
		return "NOERROR"
	case 1:
		return "FORMERR"
	case 2:
		return "SERVFAIL"
	case 3:
		return "NXDOMAIN"
	case 4:
		return "NOTIMP"
	case 5:
		return "REFUSED"
	default:
		return ""
	}
}

func AddPacketSize(f *flow.Flow, packetSize uint64) {
	if f.Extensions == nil {
		return
	}
	k := &RetinaMetadata{}      //nolint:typecheck
	f.Extensions.UnmarshalTo(k) //nolint:errcheck
	k.Bytes = packetSize
	f.Extensions, _ = anypb.New(k)
}

func PacketSize(f *flow.Flow) uint64 {
	if f.Extensions == nil {
		return 0
	}
	k := &RetinaMetadata{}      //nolint:typecheck
	f.Extensions.UnmarshalTo(k) //nolint:errcheck
	return k.Bytes
}

func AddDropReason(f *flow.Flow, dropReason uint32) {
	if f == nil {
		return
	}

	k := &RetinaMetadata{}           //nolint:typecheck // Not required to check type as we are setting it.
	f.GetExtensions().UnmarshalTo(k) //nolint:errcheck // Not required to check error as we are setting it.
	return k.GetDropReason().String()
	k.DropReason = DropReason(dropReason)
	f.Extensions, _ = anypb.New(k)

	f.Verdict = flow.Verdict_DROPPED
	f.EventType = &flow.CiliumEventType{
		Type:    int32(api.MessageTypeDrop),
		SubType: int32(api.TraceToNetwork), // This is a drop event and direction is determined later.
	}

	// Set the drop reason.
	// Retina drop reasons are different from the drop reasons available in flow library.
	// We map the ones available in flow library to the ones available in Retina.
	// Rest are set to UNKNOWN. The details are added in the metadata.
	switch k.GetDropReason() {
	case DropReason_IPTABLE_RULE_DROP:
		f.DropReasonDesc = flow.DropReason_POLICY_DENIED
	case DropReason_IPTABLE_NAT_DROP:
		f.DropReasonDesc = flow.DropReason_SNAT_NO_MAP_FOUND
	case DropReason_CONNTRACK_ADD_DROP:
		f.DropReasonDesc = flow.DropReason_UNKNOWN_CONNECTION_TRACKING_STATE
	default:
		f.DropReasonDesc = flow.DropReason_DROP_REASON_UNKNOWN
	}

	// Deprecated upstream. Will be removed in the future.
	f.DropReason = uint32(f.GetDropReasonDesc()) //nolint:staticcheck // Deprecated upstream. Will be removed in the future.:w

}

func DropReasonDescription(f *flow.Flow) string {
	if f == nil {
		return ""
	}
	k := &RetinaMetadata{}           //nolint:typecheck // Not required to check type as we are setting it.
	f.GetExtensions().UnmarshalTo(k) //nolint:errcheck // Not required to check error as we are setting it.
	return k.GetDropReason().String()
}

func decodeTime(nanoseconds int64) (pbTime *timestamppb.Timestamp, err error) {
	goTime, err := time.Parse(time.RFC3339Nano, time.Unix(0, nanoseconds).Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	pbTime = timestamppb.New(goTime)
	if err = pbTime.CheckValid(); err != nil {
		return nil, err
	}
	return pbTime, nil
}
