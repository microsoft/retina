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
	l *log.ZapLogger,
	ts int64,
	sourceIP, destIP net.IP,
	sourcePort, destPort uint32,
	proto uint8,
	observationPoint uint8,
	verdict flow.Verdict,
) *flow.Flow { //nolint:typecheck
	var l4 *flow.Layer4
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

	var (
		checkpoint   flow.TraceObservationPoint
		subeventtype int
		direction    flow.TrafficDirection
	)
	// We are attaching the filters to the veth interface on the host side.
	// So for HOST -> CONTAINER, egress of host veth is ingress of container.
	// Hence, we need to swap the direction.
	switch observationPoint {
	case uint8(0): //nolint:gomnd // flow.TraceObservationPoint_TO_STACK
		checkpoint = flow.TraceObservationPoint_TO_STACK
		direction = flow.TrafficDirection_EGRESS
		subeventtype = int(api.TraceToStack)
	case uint8(1): //nolint:gomnd // flow.TraceObservationPoint_TO_ENDPOINT
		checkpoint = flow.TraceObservationPoint_TO_ENDPOINT
		direction = flow.TrafficDirection_INGRESS
		subeventtype = int(api.TraceToLxc)
	case uint8(2): //nolint:gomnd // flow.TraceObservationPoint_FROM_NETWORK
		checkpoint = flow.TraceObservationPoint_FROM_NETWORK
		direction = flow.TrafficDirection_INGRESS
		subeventtype = int(api.TraceFromNetwork)
	case uint8(3): //nolint:gomnd // flow.TraceObservationPoint_TO_NETWORK
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
		// Packetparser running with conntrack can determine the traffic direction correctly and will override this value.
		TrafficDirection: direction,
		Verdict:          verdict,
		Extensions:       ext,
		// Setting IsReply to false by default.
		// Packetparser running with conntrack can determine the direction of the flow, and will override this value.
		IsReply: &wrapperspb.BoolValue{Value: false},
	}
	if t, err := decodeTime(ts); err == nil {
		f.Time = t
	} else {
		l.Warn("Failed to get current time", zap.Error(err))
	}
	return f
}

// AddRetinaMetadata adds the RetinaMetadata to the flow's extensions field.
func AddRetinaMetadata(f *flow.Flow, meta *RetinaMetadata) {
	ext, _ := anypb.New(meta)
	f.Extensions = ext
}

func AddTCPFlags(f *flow.Flow, syn, ack, fin, rst, psh, urg uint16) {
	if f.GetL4().GetTCP() == nil {
		return
	}

	f.GetL4().GetTCP().Flags = &flow.TCPFlags{
		SYN: syn == uint16(1),
		ACK: ack == uint16(1),
		FIN: fin == uint16(1),
		RST: rst == uint16(1),
		PSH: psh == uint16(1),
		URG: urg == uint16(1),
	}
}

func AddTCPFlagsBool(f *flow.Flow, syn, ack, fin, rst, psh, urg bool) {
	if f.GetL4().GetTCP() == nil {
		return
	}

	f.L4.GetTCP().Flags = &flow.TCPFlags{
		SYN: syn,
		ACK: ack,
		FIN: fin,
		RST: rst,
		PSH: psh,
		URG: urg,
	}
}

// Add TSval/TSecr to the flow's metadata as TCP ID.
// The TSval/TSecr works as ID for the flow.
// We will use this ID to calculate latency.
func AddTCPID(meta *RetinaMetadata, id uint64) {
	if meta == nil {
		return
	}
	meta.TcpId = id
}

func GetTCPID(f *flow.Flow) uint64 {
	if f.GetL4() == nil || f.GetL4().GetTCP() == nil {
		return 0
	}
	k := &RetinaMetadata{}      //nolint:typecheck
	f.Extensions.UnmarshalTo(k) //nolint:errcheck
	return k.TcpId
}

// AddDNSInfo adds DNS information to the flow's metadata.
func AddDNSInfo(f *flow.Flow, meta *RetinaMetadata, qType string, rCode uint32, query string, qTypes []string, numAnswers int, ips []string) {
	if f == nil || meta == nil {
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
	switch qType {
	case "Q":
		meta.DnsType = DNSType_QUERY
		f.L7.Type = flow.L7FlowType_REQUEST
	case "R":
		meta.DnsType = DNSType_RESPONSE
		f.L7.Type = flow.L7FlowType_RESPONSE
		f.IsReply = &wrapperspb.BoolValue{Value: true} // we can definitely say that this is a reply
	default:
		meta.DnsType = DNSType_UNKNOWN
		f.L7.Type = flow.L7FlowType_UNKNOWN_L7_TYPE
	}
	meta.NumResponses = uint32(numAnswers)
}

func GetDNS(f *flow.Flow) (*flow.DNS, DNSType, uint32) {
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
func DNSRcodeToString(f *flow.Flow) string {
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

// AddPacketSize adds the packet size to the flow's metadata.
func AddPacketSize(meta *RetinaMetadata, packetSize uint32) {
	if meta == nil {
		return
	}
	meta.Bytes = packetSize
}

func PacketSize(f *flow.Flow) uint32 {
	if f.Extensions == nil {
		return 0
	}
	k := &RetinaMetadata{}      //nolint:typecheck
	f.Extensions.UnmarshalTo(k) //nolint:errcheck
	return k.Bytes
}

func SourceZone(f *flow.Flow) string {
	if f.GetExtensions() == nil {
		return "unknown"
	}
	k := &RetinaMetadata{}           //nolint:typecheck // Not required to check type as we are setting it.
	f.GetExtensions().UnmarshalTo(k) //nolint:errcheck // Not required to check error as we are setting it.
	return k.GetSourceZone()
}

func DestinationZone(f *flow.Flow) string {
	if f.GetExtensions() == nil {
		return "unknown"
	}
	k := &RetinaMetadata{}           //nolint:typecheck // Not required to check type as we are setting it.
	f.GetExtensions().UnmarshalTo(k) //nolint:errcheck // Not required to check error as we are setting it.
	return k.GetDestinationZone()
}

func AddZones(f *flow.Flow, srcZone, dstZone string) {
	if f.GetExtensions() == nil {
		return
	}
	k := &RetinaMetadata{}           //nolint:typecheck // Not required to check type as we are setting it.
	f.GetExtensions().UnmarshalTo(k) //nolint:errcheck // Not required to check error as we are setting it.
	k.SourceZone = srcZone
	k.DestinationZone = dstZone
	AddRetinaMetadata(f, k)
}

// AddDropReason adds the drop reason to the flow's metadata.
func AddDropReason(f *flow.Flow, meta *RetinaMetadata, dropReason uint16) {
	if f == nil || meta == nil {
		return
	}

	meta.DropReason = DropReason(dropReason)

	f.Verdict = flow.Verdict_DROPPED

	// Set the drop reason.
	// Retina drop reasons are different from the drop reasons available in flow library.
	// We map the ones available in flow library to the ones available in Retina.
	// Rest are set to UNKNOWN. The details are added in the metadata.
	f.DropReasonDesc = GetDropReasonDesc(meta.GetDropReason())

	f.EventType = &flow.CiliumEventType{
		Type:    int32(api.MessageTypeDrop),
		SubType: int32(f.GetDropReasonDesc()), // This is the drop reason.
	}
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
