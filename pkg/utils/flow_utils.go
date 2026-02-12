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
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Additional Verdicts to be used for flow objects
const (
	Verdict_RETRANSMISSION flow.Verdict = 15
	Verdict_DNS            flow.Verdict = 16
	TypeUrl                string       = "retina.sh"
)

// Extension field keys for structpb.Struct
const (
	ExtKeyBytes                = "bytes"
	ExtKeyDNSType              = "dns_type"
	ExtKeyNumResponses         = "num_responses"
	ExtKeyTCPID                = "tcp_id"
	ExtKeyDropReason           = "drop_reason"
	ExtKeyPrevObservedPackets  = "previously_observed_packets"
	ExtKeyPrevObservedBytes    = "previously_observed_bytes"
	ExtKeyPrevObservedTCPFlags = "previously_observed_tcp_flags"
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

// NewExtensions creates a new structpb.Struct for use as flow extensions.
func NewExtensions() *structpb.Struct {
	return &structpb.Struct{Fields: make(map[string]*structpb.Value)}
}

// SetExtensions wraps the struct in Any and sets it on the flow.
// Only call this after populating all extension fields.
func SetExtensions(f *flow.Flow, s *structpb.Struct) {
	if f == nil || s == nil || len(s.GetFields()) == 0 {
		return
	}
	ext, _ := anypb.New(s)
	f.Extensions = ext
}

// getExtensionsStruct extracts the structpb.Struct from flow.Extensions.
// Returns nil if Extensions is nil or not a Struct.
func getExtensionsStruct(f *flow.Flow) *structpb.Struct {
	if f == nil || f.GetExtensions() == nil {
		return nil
	}
	s := &structpb.Struct{}
	if err := f.GetExtensions().UnmarshalTo(s); err != nil {
		return nil
	}
	return s
}

func AddTCPFlags(f *flow.Flow, syn, ack, fin, rst, psh, urg, ece, cwr, ns uint16) {
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
		ECE: ece == uint16(1),
		CWR: cwr == uint16(1),
		NS:  ns == uint16(1),
	}
}

// AddPreviouslyObservedTCPFlags adds the previously observed TCP flags to the flow's extensions.
func AddPreviouslyObservedTCPFlags(s *structpb.Struct, syn, ack, fin, rst, psh, urg, ece, cwr, ns uint32) {
	if s == nil {
		return
	}
	// Only add if at least one flag is non-zero
	if syn == 0 && ack == 0 && fin == 0 && rst == 0 && psh == 0 && urg == 0 && ece == 0 && cwr == 0 && ns == 0 {
		return
	}
	tcpFlags := &structpb.Struct{Fields: map[string]*structpb.Value{
		SYN: structpb.NewNumberValue(float64(syn)),
		ACK: structpb.NewNumberValue(float64(ack)),
		FIN: structpb.NewNumberValue(float64(fin)),
		RST: structpb.NewNumberValue(float64(rst)),
		PSH: structpb.NewNumberValue(float64(psh)),
		URG: structpb.NewNumberValue(float64(urg)),
		ECE: structpb.NewNumberValue(float64(ece)),
		CWR: structpb.NewNumberValue(float64(cwr)),
		NS:  structpb.NewNumberValue(float64(ns)),
	}}
	s.Fields[ExtKeyPrevObservedTCPFlags] = structpb.NewStructValue(tcpFlags)
}

func PreviouslyObservedTCPFlags(f *flow.Flow) map[string]uint32 {
	s := getExtensionsStruct(f)
	if s == nil {
		return nil
	}
	v, ok := s.GetFields()[ExtKeyPrevObservedTCPFlags]
	if !ok || v.GetStructValue() == nil {
		return nil
	}
	result := make(map[string]uint32)
	for k, val := range v.GetStructValue().GetFields() {
		result[k] = uint32(val.GetNumberValue())
	}
	return result
}

// AddPreviouslyObservedBytes adds the previously observed bytes to the flow's extensions.
func AddPreviouslyObservedBytes(s *structpb.Struct, bytes uint32) {
	if s == nil || bytes == 0 {
		return
	}
	s.Fields[ExtKeyPrevObservedBytes] = structpb.NewNumberValue(float64(bytes))
}

func PreviouslyObservedBytes(f *flow.Flow) uint32 {
	s := getExtensionsStruct(f)
	if s == nil {
		return 0
	}
	v, ok := s.GetFields()[ExtKeyPrevObservedBytes]
	if !ok {
		return 0
	}
	return uint32(v.GetNumberValue())
}

// AddPreviouslyObservedPackets adds the previously observed packets to the flow's extensions.
func AddPreviouslyObservedPackets(s *structpb.Struct, packets uint32) {
	if s == nil || packets == 0 {
		return
	}
	s.Fields[ExtKeyPrevObservedPackets] = structpb.NewNumberValue(float64(packets))
}

func PreviouslyObservedPackets(f *flow.Flow) uint32 {
	s := getExtensionsStruct(f)
	if s == nil {
		return 0
	}
	v, ok := s.GetFields()[ExtKeyPrevObservedPackets]
	if !ok {
		return 0
	}
	return uint32(v.GetNumberValue())
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

// AddTCPID adds TSval/TSecr to the flow's extensions as TCP ID.
// The TSval/TSecr works as ID for the flow.
// We will use this ID to calculate latency.
func AddTCPID(s *structpb.Struct, id uint64) {
	if s == nil || id == 0 {
		return
	}
	s.Fields[ExtKeyTCPID] = structpb.NewNumberValue(float64(id))
}

func GetTCPID(f *flow.Flow) uint64 {
	if f.GetL4() == nil || f.GetL4().GetTCP() == nil {
		return 0
	}
	s := getExtensionsStruct(f)
	if s == nil {
		return 0
	}
	v, ok := s.GetFields()[ExtKeyTCPID]
	if !ok {
		return 0
	}
	return uint64(v.GetNumberValue())
}

// AddDNSInfo adds DNS information to the flow and its extensions.
func AddDNSInfo(f *flow.Flow, s *structpb.Struct, qType string, rCode uint32, query string, qTypes []string, numAnswers int, ips []string) {
	if f == nil || s == nil {
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
		s.Fields[ExtKeyDNSType] = structpb.NewStringValue(DNSType_QUERY.String())
		f.L7.Type = flow.L7FlowType_REQUEST
	case "R":
		s.Fields[ExtKeyDNSType] = structpb.NewStringValue(DNSType_RESPONSE.String())
		f.L7.Type = flow.L7FlowType_RESPONSE
		f.IsReply = &wrapperspb.BoolValue{Value: true} // we can definitely say that this is a reply
	default:
		f.L7.Type = flow.L7FlowType_UNKNOWN_L7_TYPE
	}
	if numAnswers > 0 {
		s.Fields[ExtKeyNumResponses] = structpb.NewNumberValue(float64(numAnswers))
	}
}

func GetDNS(f *flow.Flow) (*flow.DNS, DNSType, uint32) {
	if f == nil || f.L7 == nil || f.L7.GetDns() == nil {
		return nil, DNSType_UNKNOWN, 0
	}
	dns := f.L7.GetDns()
	s := getExtensionsStruct(f)
	if s == nil {
		return dns, DNSType_UNKNOWN, 0
	}

	dnsType := DNSType_UNKNOWN
	if v, ok := s.GetFields()[ExtKeyDNSType]; ok {
		switch v.GetStringValue() {
		case DNSType_QUERY.String():
			dnsType = DNSType_QUERY
		case DNSType_RESPONSE.String():
			dnsType = DNSType_RESPONSE
		}
	}

	var numResponses uint32
	if v, ok := s.GetFields()[ExtKeyNumResponses]; ok {
		numResponses = uint32(v.GetNumberValue())
	}

	return dns, dnsType, numResponses
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

// AddPacketSize adds the packet size to the flow's extensions.
func AddPacketSize(s *structpb.Struct, packetSize uint32) {
	if s == nil || packetSize == 0 {
		return
	}
	s.Fields[ExtKeyBytes] = structpb.NewNumberValue(float64(packetSize))
}

func PacketSize(f *flow.Flow) uint32 {
	s := getExtensionsStruct(f)
	if s == nil {
		return 0
	}
	v, ok := s.GetFields()[ExtKeyBytes]
	if !ok {
		return 0
	}
	return uint32(v.GetNumberValue())
}

// AddDropReason adds the drop reason to the flow and its extensions.
func AddDropReason(f *flow.Flow, s *structpb.Struct, dropReason uint16) {
	if f == nil || s == nil {
		return
	}

	dr := DropReason(dropReason)
	s.Fields[ExtKeyDropReason] = structpb.NewStringValue(dr.String())

	f.Verdict = flow.Verdict_DROPPED

	// Set the drop reason.
	// Retina drop reasons are different from the drop reasons available in flow library.
	// We map the ones available in flow library to the ones available in Retina.
	// Rest are set to UNKNOWN. The details are added in the extensions.
	f.DropReasonDesc = GetDropReasonDesc(dr)

	f.EventType = &flow.CiliumEventType{
		Type:    int32(api.MessageTypeDrop),
		SubType: int32(f.GetDropReasonDesc()), // This is the drop reason.
	}
}

func DropReasonDescription(f *flow.Flow) string {
	if f == nil {
		return ""
	}
	s := getExtensionsStruct(f)
	if s == nil {
		return ""
	}
	v, ok := s.GetFields()[ExtKeyDropReason]
	if !ok {
		return ""
	}
	return v.GetStringValue()
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
