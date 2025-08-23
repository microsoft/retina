// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Hubble

package ebpfwindows

import (
	errorTypes "errors"
	"fmt"
	"log/slog"
	"math"
	"net/netip"
	"strings"

	pb "github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	observerTypes "github.com/cilium/cilium/pkg/hubble/observer/types"
	"github.com/cilium/cilium/pkg/hubble/parser/errors"
	"github.com/cilium/cilium/pkg/hubble/parser/options"
	"github.com/cilium/cilium/pkg/lock"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"go4.org/netipx"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const MaxInt = int(^uint(0) >> 1)

// Parser is a parser for L3/L4 payloads
type Parser struct {
	log                 *slog.Logger
	epResolver          *EndpointResolver
	correlateL3L4Policy bool
	packet              *packet
}

var (
	errDataOffsetTooLarge = errorTypes.New("data offset too large")
	errNotEnoughBytes     = errorTypes.New("not enough bytes to decode")
	errDropReasonOverflow = errorTypes.New("drop reason exceeds int32 range")
)

// re-usable packet to avoid reallocating gopacket datastructures
type packet struct {
	lock.Mutex
	decLayerL2Dev *gopacket.DecodingLayerParser
	decLayerL3Dev struct {
		IPv4 *gopacket.DecodingLayerParser
		IPv6 *gopacket.DecodingLayerParser
	}

	Layers []gopacket.LayerType
	layers.Ethernet
	layers.IPv4
	layers.IPv6
	layers.ICMPv4
	layers.ICMPv6
	layers.TCP
	layers.UDP
	layers.SCTP
}

// New returns a new L3/L4 parser
func NewParser(
	log *slog.Logger,
	opts ...options.Option,
) (*Parser, error) {
	packet := &packet{}
	decoders := []gopacket.DecodingLayer{
		&packet.Ethernet,
		&packet.IPv4, &packet.IPv6,
		&packet.ICMPv4, &packet.ICMPv6,
		&packet.TCP, &packet.UDP, &packet.SCTP,
	}
	packet.decLayerL2Dev = gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, decoders...)
	packet.decLayerL3Dev.IPv4 = gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, decoders...)
	packet.decLayerL3Dev.IPv6 = gopacket.NewDecodingLayerParser(layers.LayerTypeIPv6, decoders...)
	// Let packet.decLayer.DecodeLayers return a nil error when it
	// encounters a layer it doesn't have a parser for, instead of returning
	// an UnsupportedLayerType error.
	packet.decLayerL2Dev.IgnoreUnsupported = true
	packet.decLayerL3Dev.IPv4.IgnoreUnsupported = true
	packet.decLayerL3Dev.IPv6.IgnoreUnsupported = true

	args := &options.Options{
		EnableNetworkPolicyCorrelation: true,
	}

	for _, opt := range opts {
		opt(args)
	}

	return &Parser{
		log:                 log,
		epResolver:          NewEndpointResolver(log),
		packet:              packet,
		correlateL3L4Policy: args.EnableNetworkPolicyCorrelation,
	}, nil
}

// Decode decodes a cilium monitor 'payload' and returns a v1.Event with
// the Event field populated.
func (p *Parser) Decode(monitorEvent *observerTypes.MonitorEvent) (*v1.Event, error) {
	if monitorEvent == nil {
		return nil, errors.ErrEmptyData
	}

	// TODO: Pool decoded flows instead of allocating new objects each time.
	ts := timestamppb.New(monitorEvent.Timestamp)
	ev := &v1.Event{
		Timestamp: ts,
	}

	switch payload := monitorEvent.Payload.(type) {
	case *observerTypes.PerfEvent:
		if len(payload.Data) == 0 {
			return nil, errors.ErrEmptyData
		}

		flow := &pb.Flow{}
		switch payload.Data[0] {
		case monitorAPI.MessageTypeDebug:
			return nil, errors.ErrUnknownEventType
		case monitorAPI.MessageTypeTraceSock:
			return nil, errors.ErrUnknownEventType
		default:
			if err := p.decode(payload.Data, flow); err != nil {
				return nil, err
			}
		}
		flow.Uuid = monitorEvent.UUID.String()
		// FIXME: Time and NodeName are now part of GetFlowsResponse. We
		// populate these fields for compatibility with old clients.
		flow.Time = ts
		flow.NodeName = monitorEvent.NodeName
		ev.Event = flow
		return ev, nil
	case nil:
		return ev, errors.ErrEmptyData
	default:
		return nil, errors.ErrUnknownEventType
	}
}

const MessageTypePktmonDrop = 100

// Decode decodes the data from 'data' into 'decoded'
func (p *Parser) decode(data []byte, decoded *pb.Flow) error {
	if len(data) == 0 {
		return errors.ErrEmptyData
	}

	eventType := data[0]

	var packetOffset int
	var offset uint
	var dn *DropNotify
	var tn *TraceNotify
	var eventSubType uint8
	var authType pb.AuthType

	switch eventType {
	case monitorAPI.MessageTypeDrop:
		dn = &DropNotify{}
		if err := DecodeDropNotify(data, dn); err != nil {
			return fmt.Errorf("failed to parse drop: %w", err)
		}
		eventSubType = dn.SubType
		offset = dn.DataOffset()
		if offset > uint(MaxInt) {
			return fmt.Errorf("%w: %d", errDataOffsetTooLarge, offset)
		}
		packetOffset = int(offset)
	case monitorAPI.MessageTypeTrace:
		tn = &TraceNotify{}
		if err := DecodeTraceNotify(data, tn); err != nil {
			return fmt.Errorf("failed to parse trace: %w", err)
		}
		eventSubType = tn.ObsPoint

		if tn.ObsPoint != 0 {
			decoded.TraceObservationPoint = pb.TraceObservationPoint(tn.ObsPoint)
		} else {
			// specifically handle the zero value in the observation enum so the json
			// export and the API don't carry extra meaning with the zero value
			decoded.TraceObservationPoint = pb.TraceObservationPoint_TO_ENDPOINT
		}

		offset = tn.DataOffset()
		if offset > uint(MaxInt) {
			return fmt.Errorf("%w: %d", errDataOffsetTooLarge, offset)
		}

		packetOffset = int(offset)

	case MessageTypePktmonDrop:
		dn = &DropNotify{}
		if err := DecodePktmonDrop(data, dn); err != nil {
			return fmt.Errorf("failed to parse pktmon drop: %w", err)
		}

	default:
		return fmt.Errorf("invalid event type: %w", errors.NewErrInvalidType(eventType))
	}

	if len(data) < packetOffset {
		return fmt.Errorf("%w: %d", errNotEnoughBytes, data)
	}

	p.packet.Lock()
	defer p.packet.Unlock()

	// Since v1.1.18, DecodeLayers returns a non-nil error for an empty packet, see
	// https://github.com/google/gopacket/issues/846
	// TODO: reconsider this check if the issue is fixed upstream
	if len(data[packetOffset:]) > 0 {
		var isL3Device, isIPv6 bool
		if (tn != nil && tn.IsL3Device()) || (dn != nil && dn.IsL3Device()) {
			isL3Device = true
		}
		if tn != nil && tn.IsIPv6() || (dn != nil && dn.IsIPv6()) {
			isIPv6 = true
		}

		var err error
		switch {
		case !isL3Device:
			err = p.packet.decLayerL2Dev.DecodeLayers(data[packetOffset:], &p.packet.Layers)
		case isIPv6:
			err = p.packet.decLayerL3Dev.IPv6.DecodeLayers(data[packetOffset:], &p.packet.Layers)
		default:
			err = p.packet.decLayerL3Dev.IPv4.DecodeLayers(data[packetOffset:], &p.packet.Layers)
		}

		if err != nil {
			return fmt.Errorf("decode layers failed: %w", err)
		}
	} else {
		// Truncate layers to avoid accidental re-use.
		p.packet.Layers = p.packet.Layers[:0]
	}

	decodedpacket := decodeLayers(p.packet)
	srcIP := decodedpacket.SourceIP
	ip := decodedpacket.IP

	if tn != nil && decodedpacket.IP != nil {
		if !tn.OriginalIP().IsUnspecified() {
			// Ignore invalid IP - getters will handle invalid value.
			srcIP, _ = netipx.FromStdIP(tn.OriginalIP())
			// On SNAT the trace notification has OrigIP set to the pre
			// translation IP and the source IP parsed from the header is the
			// post translation IP. The check is here because sometimes we get
			// trace notifications with OrigIP set to the header's IP
			// (pre-translation events?)
			if ip.GetSource() != srcIP.String() {
				ip.SourceXlated = ip.GetSource()
				ip.Source = srcIP.String()
			}
		}

		ip.Encrypted = tn.IsEncrypted()
	}

	srcLabelID, dstLabelID := decodeSecurityIdentities(dn, tn)
	datapathContext := DatapathContext{
		SrcIP:                 srcIP,
		SrcLabelID:            srcLabelID,
		DstIP:                 decodedpacket.DestinationIP,
		DstLabelID:            dstLabelID,
		TraceObservationPoint: decoded.GetTraceObservationPoint(),
	}
	srcEndpoint := p.epResolver.ResolveEndpoint(srcIP, srcLabelID, datapathContext)
	dstEndpoint := p.epResolver.ResolveEndpoint(decodedpacket.DestinationIP, dstLabelID, datapathContext)

	decoded.Verdict = decodeVerdict(dn, tn)
	decoded.AuthType = authType
	//nolint:staticcheck // SA1019 - temporary assignment for backward compatibility
	decoded.DropReason = decodeDropReason(dn)
	//nolint:staticcheck // SA1019 - temporary assignment for backward compatibility
	dropReason := decoded.GetDropReason()
	if dropReason > math.MaxInt32 {
		return fmt.Errorf("%w: %d", errDropReasonOverflow, dropReason)
	}
	decoded.DropReasonDesc = pb.DropReason(int32(dropReason))
	decoded.File = decodeFileInfo(dn)
	decoded.Ethernet = decodedpacket.Ethernet
	decoded.IP = decodedpacket.IP
	decoded.L4 = decodedpacket.L4
	decoded.Source = srcEndpoint
	decoded.Destination = dstEndpoint
	decoded.Type = pb.FlowType_L3_L4
	decoded.L7 = nil
	decoded.IsReply = decodeIsReply(tn)
	//nolint:staticcheck // SA1019 - temporary assignment for backward compatibility
	decoded.Reply = decoded.GetIsReply().GetValue() // false if GetIsReply() is nil
	decoded.EventType = decodeCiliumEventType(eventType, eventSubType)
	decoded.TraceReason = decodeTraceReason(tn)
	decoded.Interface = p.decodeNetworkInterface(tn)
	decoded.ProxyPort = decodeProxyPort(tn)
	//nolint:staticcheck // SA1019 - temporary assignment for backward compatibility
	decoded.Summary = decodedpacket.Summary

	return nil
}

type DecodedPacket struct {
	Ethernet        *pb.Ethernet
	IP              *pb.IP
	L4              *pb.Layer4
	SourceIP        netip.Addr
	DestinationIP   netip.Addr
	SourcePort      uint16
	DestinationPort uint16
	Summary         string
}

func decodeLayers(packet *packet) *DecodedPacket {
	var (
		ethernet        *pb.Ethernet
		ip              *pb.IP
		l4              *pb.Layer4
		sourceIP        netip.Addr
		destinationIP   netip.Addr
		sourcePort      uint16
		destinationPort uint16
		summary         string
	)

	for _, typ := range packet.Layers {
		summary = typ.String()
		switch typ {
		case layers.LayerTypeEthernet:
			ethernet = decodeEthernet(&packet.Ethernet)
		case layers.LayerTypeIPv4:
			ip, sourceIP, destinationIP = decodeIPv4(&packet.IPv4)
		case layers.LayerTypeIPv6:
			ip, sourceIP, destinationIP = decodeIPv6(&packet.IPv6)
		case layers.LayerTypeTCP:
			l4, sourcePort, destinationPort = decodeTCP(&packet.TCP)
			summary = "TCP Flags: " + getTCPFlags(packet.TCP)
		case layers.LayerTypeUDP:
			l4, sourcePort, destinationPort = decodeUDP(&packet.UDP)
		case layers.LayerTypeSCTP:
			l4, sourcePort, destinationPort = decodeSCTP(&packet.SCTP)
		case layers.LayerTypeICMPv4:
			l4 = decodeICMPv4(&packet.ICMPv4)
			summary = "ICMPv4 " + packet.ICMPv4.TypeCode.String()
		case layers.LayerTypeICMPv6:
			l4 = decodeICMPv6(&packet.ICMPv6)
			summary = "ICMPv6 " + packet.ICMPv6.TypeCode.String()
		}
	}

	return &DecodedPacket{
		Ethernet:        ethernet,
		IP:              ip,
		L4:              l4,
		SourceIP:        sourceIP,
		DestinationIP:   destinationIP,
		SourcePort:      sourcePort,
		DestinationPort: destinationPort,
		Summary:         summary,
	}
}

func decodeVerdict(dn *DropNotify, tn *TraceNotify) pb.Verdict {
	switch {
	case dn != nil:
		return pb.Verdict_DROPPED
	case tn != nil:
		return pb.Verdict_FORWARDED
	}
	return pb.Verdict_VERDICT_UNKNOWN
}

func decodeDropReason(dn *DropNotify) uint32 {
	if dn != nil {
		return uint32(dn.SubType)
	}
	return 0
}

func decodeFileInfo(dn *DropNotify) *pb.FileInfo {
	if dn != nil {
		return &pb.FileInfo{
			Name: monitorAPI.BPFFileName(dn.File),
			Line: uint32(dn.Line),
		}
	}
	return nil
}

func decodeEthernet(ethernet *layers.Ethernet) *pb.Ethernet {
	return &pb.Ethernet{
		Source:      ethernet.SrcMAC.String(),
		Destination: ethernet.DstMAC.String(),
	}
}

func decodeIPv4(ipv4 *layers.IPv4) (ip *pb.IP, src, dst netip.Addr) {
	// Ignore invalid IPs - getters will handle invalid values.
	// IPs can be empty for Ethernet-only packets.
	src, _ = netipx.FromStdIP(ipv4.SrcIP)
	dst, _ = netipx.FromStdIP(ipv4.DstIP)
	return &pb.IP{
		Source:      ipv4.SrcIP.String(),
		Destination: ipv4.DstIP.String(),
		IpVersion:   pb.IPVersion_IPv4,
	}, src, dst
}

func decodeIPv6(ipv6 *layers.IPv6) (ip *pb.IP, src, dst netip.Addr) {
	// Ignore invalid IPs - getters will handle invalid values.
	// IPs can be empty for Ethernet-only packets.
	src, _ = netipx.FromStdIP(ipv6.SrcIP)
	dst, _ = netipx.FromStdIP(ipv6.DstIP)
	return &pb.IP{
		Source:      ipv6.SrcIP.String(),
		Destination: ipv6.DstIP.String(),
		IpVersion:   pb.IPVersion_IPv6,
	}, src, dst
}

func decodeTCP(tcp *layers.TCP) (l4 *pb.Layer4, src, dst uint16) {
	return &pb.Layer4{
		Protocol: &pb.Layer4_TCP{
			TCP: &pb.TCP{
				SourcePort:      uint32(tcp.SrcPort),
				DestinationPort: uint32(tcp.DstPort),
				Flags: &pb.TCPFlags{
					FIN: tcp.FIN, SYN: tcp.SYN, RST: tcp.RST,
					PSH: tcp.PSH, ACK: tcp.ACK, URG: tcp.URG,
					ECE: tcp.ECE, CWR: tcp.CWR, NS: tcp.NS,
				},
			},
		},
	}, uint16(tcp.SrcPort), uint16(tcp.DstPort)
}

func decodeSCTP(sctp *layers.SCTP) (l4 *pb.Layer4, src, dst uint16) {
	return &pb.Layer4{
		Protocol: &pb.Layer4_SCTP{
			SCTP: &pb.SCTP{
				SourcePort:      uint32(sctp.SrcPort),
				DestinationPort: uint32(sctp.DstPort),
			},
		},
	}, uint16(sctp.SrcPort), uint16(sctp.DstPort)
}

func decodeUDP(udp *layers.UDP) (l4 *pb.Layer4, src, dst uint16) {
	return &pb.Layer4{
		Protocol: &pb.Layer4_UDP{
			UDP: &pb.UDP{
				SourcePort:      uint32(udp.SrcPort),
				DestinationPort: uint32(udp.DstPort),
			},
		},
	}, uint16(udp.SrcPort), uint16(udp.DstPort)
}

func decodeICMPv4(icmp *layers.ICMPv4) *pb.Layer4 {
	return &pb.Layer4{
		Protocol: &pb.Layer4_ICMPv4{ICMPv4: &pb.ICMPv4{
			Type: uint32(icmp.TypeCode.Type()),
			Code: uint32(icmp.TypeCode.Code()),
		}},
	}
}

func decodeICMPv6(icmp *layers.ICMPv6) *pb.Layer4 {
	return &pb.Layer4{
		Protocol: &pb.Layer4_ICMPv6{ICMPv6: &pb.ICMPv6{
			Type: uint32(icmp.TypeCode.Type()),
			Code: uint32(icmp.TypeCode.Code()),
		}},
	}
}

func decodeIsReply(tn *TraceNotify) *wrapperspb.BoolValue {
	switch {
	case tn != nil && tn.TraceReasonIsKnown():
		if tn.TraceReasonIsEncap() || tn.TraceReasonIsDecap() {
			return nil
		}
		// Reason was specified by the datapath, just reuse it.
		return &wrapperspb.BoolValue{
			Value: tn.TraceReasonIsReply(),
		}
	default:
		// For other events, such as drops, we simply do not know if they were
		// replies or not.
		return nil
	}
}

func decodeCiliumEventType(eventType, eventSubType uint8) *pb.CiliumEventType {
	return &pb.CiliumEventType{
		Type:    int32(eventType),
		SubType: int32(eventSubType),
	}
}

func decodeTraceReason(tn *TraceNotify) pb.TraceReason {
	if tn == nil {
		return pb.TraceReason_TRACE_REASON_UNKNOWN
	}
	// The Hubble protobuf enum values aren't 1:1 mapped with Cilium's datapath
	// because we want pb.TraceReason_TRACE_REASON_UNKNOWN = 0 while in
	// datapath monitor.TraceReasonUnknown = 5. The mapping works as follow:
	switch {
	// monitor.TraceReasonUnknown is mapped to pb.TraceReason_TRACE_REASON_UNKNOWN
	case tn.TraceReason() == TraceReasonUnknown:
		return pb.TraceReason_TRACE_REASON_UNKNOWN
	// values before monitor.TraceReasonUnknown are "offset by one", e.g.
	// TraceReasonCtEstablished = 1 → TraceReason_ESTABLISHED = 2 to make room
	// for the zero value.
	case tn.TraceReason() < TraceReasonUnknown:
		return pb.TraceReason(tn.TraceReason()) + 1
	// all values greater than monitor.TraceReasonUnknown are mapped 1:1 with
	// the datapath values.
	default:
		return pb.TraceReason(tn.TraceReason())
	}
}

func decodeSecurityIdentities(dn *DropNotify, tn *TraceNotify) (
	sourceSecurityIdentiy, destinationSecurityIdentity uint32,
) {
	switch {
	case dn != nil:
		sourceSecurityIdentiy = uint32(dn.SrcLabel)
		destinationSecurityIdentity = uint32(dn.DstLabel)
	case tn != nil:
		sourceSecurityIdentiy = uint32(tn.SrcLabel)
		destinationSecurityIdentity = uint32(tn.DstLabel)
	}

	return
}

func getTCPFlags(tcp layers.TCP) string {
	const (
		syn         = "SYN"
		ack         = "ACK"
		rst         = "RST"
		fin         = "FIN"
		psh         = "PSH"
		urg         = "URG"
		ece         = "ECE"
		cwr         = "CWR"
		ns          = "NS"
		maxTCPFlags = 9
		comma       = ", "
	)

	info := make([]string, 0, maxTCPFlags)

	if tcp.SYN {
		info = append(info, syn)
	}

	if tcp.ACK {
		info = append(info, ack)
	}

	if tcp.RST {
		info = append(info, rst)
	}

	if tcp.FIN {
		info = append(info, fin)
	}

	if tcp.PSH {
		info = append(info, psh)
	}

	if tcp.URG {
		info = append(info, urg)
	}

	if tcp.ECE {
		info = append(info, ece)
	}

	if tcp.CWR {
		info = append(info, cwr)
	}

	if tcp.NS {
		info = append(info, ns)
	}

	return strings.Join(info, comma)
}

func (p *Parser) decodeNetworkInterface(tn *TraceNotify) *pb.NetworkInterface {
	ifIndex := uint32(0)
	if tn != nil {
		ifIndex = tn.Ifindex
	}

	if ifIndex == 0 {
		return nil
	}

	var name string
	return &pb.NetworkInterface{
		Index: ifIndex,
		Name:  name,
	}
}

func decodeProxyPort(tn *TraceNotify) uint32 {
	if tn != nil && tn.ObsPoint == monitorAPI.TraceToProxy {
		return uint32(tn.DstID)
	}
	return 0
}
