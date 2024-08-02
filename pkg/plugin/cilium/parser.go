package cilium

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/byteorder"
	parserErrors "github.com/cilium/cilium/pkg/hubble/parser/errors"
	"github.com/cilium/cilium/pkg/monitor"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/monitor/payload"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const ErrNotImplemented = "Error, not implemented for type: %s"

var (
	ErrEmptyData    = errors.New("empty data")
	ErrNoPacketData = errors.New("no packet data")
	ErrNoLayer4     = errors.New("no layer 4")
)

func (p *parser) Init() {
	packet := &packet{}
	packet.decLayer = gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet, &packet.Ethernet,
		&packet.IPv4, &packet.IPv6,
		&packet.TCP, &packet.UDP,
	)
	packet.decLayer.IgnoreUnsupported = true
	p.packet = packet
}

// monitorAgent.SendEvents - is used to notify for Agent Events (Access Log (proxy) and Agent Notify (cilium agent events - crud for ep, policy, svc))
// hubble monitorConsumer.sendEvent -- (NotifyPerfEvent) this func sends a monitorEvent to the consumer from hubble monitor.
// specifically, the hubble consumer adds the event to the observer's event channel
// Agent Events
//   - MessageTypeAccessLog:		accesslog.LogRecord
//   - MessageTypeAgent:			api.AgentNotify
//
// Perf Events
//   - MessageTypeDrop:			monitor.DropNotify
//   - MessageTypeDebug:			monitor.DebugMsg
//   - MessageTypeCapture:		monitor.DebugCapture
//   - MessageTypeTrace:			monitor.TraceNotify
//   - MessageTypePolicyVerdict:	monitor.PolicyVerdictNotify
//
// Reference hubble/parser/threefour
func (p *parser) Decode(pl *payload.Payload) (*flow.Flow, error) {
	switch pl.Type {
	case payload.EventSample:
		data := pl.Data
		if len(data) == 0 {
			return nil, ErrEmptyData
		}

		var eventType uint8 = data[0]
		var packetOffset int
		var obsPoint flow.TraceObservationPoint
		var dropNotify *monitor.DropNotify
		var traceNotify *monitor.TraceNotify
		var policyVerdict *monitor.PolicyVerdictNotify
		// var debugCap *monitor.DebugCapture
		var eventSubType uint8
		// var authType flow.AuthType
		// prefix := fmt.Sprintf("CPU %02d:", pl.CPU)

		switch eventType {
		// Agent Events
		case monitorAPI.MessageTypeAccessLog:
			// These are l7 events
			// We have dns from ig. We are missing other such as kafka, icmp, etc
			// buf := bytes.NewBuffer(data[1:])
			// dec := gob.NewDecoder(buf)
			// lr := monitor.LogRecordNotify{}
			// if err := dec.Decode(&lr); err != nil {
			// 	p.l.Error("Failed to decode log record notify", zap.Error(err))
			// 	return parserErrors.ErrEventSkipped
			// }
			// p.l.Info("Access log event")
			// lr.DumpJSON()
			return nil, fmt.Errorf(ErrNotImplemented, monitorAPI.MessageTypeAccessLog)
		case monitorAPI.MessageTypeAgent:
			// buf := bytes.NewBuffer(data[1:])
			// dec := gob.NewDecoder(buf)
			// an := monitorAPI.AgentNotify{}
			// if err := dec.Decode(&an); err != nil {
			// 	p.l.Error("Failed to decode agent notify", zap.Error(err))
			// 	return parserErrors.ErrEventSkipped
			// }
			// p.l.Info("Agent event")
			// an.DumpJSON()
			return nil, fmt.Errorf(ErrNotImplemented, monitorAPI.MessageTypeAgent)
		// PerfEvents .. L34
		case monitorAPI.MessageTypeDrop:
			dn := monitor.DropNotify{}
			if err := binary.Read(bytes.NewReader(data), byteorder.Native, &dn); err != nil {
				p.l.Error("Failed to decode drop notify", zap.Error(err))
				return nil, parserErrors.ErrEventSkipped
			}
			eventSubType = dn.SubType
			packetOffset = monitor.DropNotifyLen
			dropNotify = &dn
		case monitorAPI.MessageTypeTrace:
			tn := monitor.TraceNotify{}
			if err := monitor.DecodeTraceNotify(data, &tn); err != nil {
				p.l.Error("Failed to decode trace notify", zap.Error(err))
				return nil, parserErrors.ErrEventSkipped
			}
			eventSubType = tn.ObsPoint
			if tn.ObsPoint == 0 {
				obsPoint = flow.TraceObservationPoint_TO_ENDPOINT
			} else {
				obsPoint = flow.TraceObservationPoint(tn.ObsPoint)
			}
			packetOffset = (int)(tn.DataOffset())
			traceNotify = &tn
		case monitorAPI.MessageTypePolicyVerdict:
			pn := monitor.PolicyVerdictNotify{}
			if err := binary.Read(bytes.NewReader(data), byteorder.Native, &pn); err != nil {
				p.l.Error("Failed to decode policy verdict notify", zap.Error(err))
				return nil, parserErrors.ErrEventSkipped
			}
			eventSubType = pn.SubType
			packetOffset = monitor.PolicyVerdictNotifyLen
			// authType = flow.AuthType(pn.AuthType)
			policyVerdict = &pn
			// p.l.Info("Policy verdict event")
			// pn.DumpInfo(data, false)
		case monitorAPI.MessageTypeCapture:
			// dc := monitor.DebugCapture{}
			// if err := binary.Read(bytes.NewReader(data), byteorder.Native, &dc); err != nil {
			// 	p.l.Error("Failed to decode debug capture", zap.Error(err))
			// }
			return nil, fmt.Errorf(ErrNotImplemented, monitorAPI.MessageTypeNameCapture)
		// PerfEvents
		case monitorAPI.MessageTypeDebug:
			// dm := monitor.DebugMsg{}
			// if err := binary.Read(bytes.NewReader(data), byteorder.Native, &dm); err != nil {
			// 	p.l.Error("Failed to decode debug message", zap.Error(err))
			// }
			return nil, fmt.Errorf(ErrNotImplemented, monitorAPI.MessageTypeNameDebug)
		case monitorAPI.MessageTypeTraceSock:
			// tn := monitor.TraceSockNotify{}
			// if err := binary.Read(bytes.NewReader(data), byteorder.Native, &tn); err != nil {
			// 	p.l.Error("Failed to decode trace sock notify", zap.Error(err))
			// }
			return nil, fmt.Errorf(ErrNotImplemented, monitorAPI.MessageTypeNameTraceSock)
		case monitorAPI.MessageTypeRecCapture:
			// no op for now
			// rc := monitor.RecorderCapture{}
			// if err := binary.Read(bytes.NewReader(data), byteorder.Native, &rc); err != nil {
			// 	p.l.Error("Failed to decode recorder capture", zap.Error(err))
			// }
			return nil, fmt.Errorf(ErrNotImplemented, monitorAPI.MessageTypeNameRecCapture)
		default:
			p.l.Debug("Unknown event", zap.Any("data", data))
			return nil, parserErrors.ErrUnknownEventType
		}

		p.packet.Lock()
		defer p.packet.Unlock()
		// Decode the layers
		if len(data) <= packetOffset {
			p.l.Error("No packet data found")
			return nil, ErrNoPacketData
		}

		if len(data[packetOffset:]) > 0 {
			err := p.packet.decLayer.DecodeLayers(data[packetOffset:], &p.packet.Layers)
			if err != nil {
				return nil, err
			}
		} else {
			// Truncate layers to avoid accidental re-use.
			p.packet.Layers = p.packet.Layers[:0]
		}
		eth, ip, l4, srcIP, dstIP, srcPort, dstPort := decodeLayers(p.packet)

		var protocol uint8
		if l4 != nil {
			switch l4.Protocol.(type) {
			case *flow.Layer4_TCP:
				protocol = 6
			case *flow.Layer4_UDP:
				protocol = 17
			}
		} else {
			return nil, ErrNoLayer4
		}

		newFl := utils.ToFlow(
			time.Now().Unix(),
			*srcIP,
			*dstIP,
			srcPort,
			dstPort,
			protocol,
			uint32(obsPoint),
			getVerdict(dropNotify, traceNotify, policyVerdict),
		)
		newFl.IP.IpVersion = ip.IpVersion
		newFl.EventType = &flow.CiliumEventType{
			Type:    int32(eventType),
			SubType: int32(eventSubType),
		}
		newFl.Ethernet = eth
		var drVal uint32
		switch {
		case dropNotify != nil:
			drVal = uint32(dropNotify.SubType)
		case policyVerdict != nil && policyVerdict.Verdict < 0:
			// if the flow was dropped, verdict equals the negative of the drop reason
			drVal = uint32(-policyVerdict.Verdict)
		default:
			drVal = 0
		}
		newFl.DropReason = drVal
		newFl.DropReasonDesc = flow.DropReason(newFl.DropReason)
		// Decode ICMPv4/v6
		// Decode SCTP
		// Decode ProxyPort
		// Decode NIC
		// Decode IsReply
		return newFl, nil
	case payload.RecordLost:
		p.l.Warn("Record lost for cilium event", zap.Uint64("lost", pl.Lost))
		return nil, fmt.Errorf("Record lost for cilium event: %d", pl.Lost)
	default:
		p.l.Warn("Unknown event type", zap.Int("type", pl.Type))
		return nil, parserErrors.ErrUnknownEventType
	}
}

func decodeLayers(packet *packet) (
	ethernet *flow.Ethernet,
	ip *flow.IP,
	l4 *flow.Layer4,
	sourceIP, destinationIP *net.IP,
	sourcePort, destinationPort uint32,
	// tcp summary
) {
	for _, typ := range packet.Layers {
		// summary = typ.String()
		switch typ {
		case layers.LayerTypeEthernet:
			ethernet = &flow.Ethernet{
				Source:      packet.Ethernet.SrcMAC.String(),
				Destination: packet.Ethernet.DstMAC.String(),
			}
		case layers.LayerTypeIPv4:
			sourceIP = &packet.IPv4.SrcIP
			destinationIP = &packet.IPv4.DstIP
			ip = &flow.IP{
				Source:      sourceIP.String(),
				Destination: destinationIP.String(),
				IpVersion:   flow.IPVersion_IPv4,
			}
		case layers.LayerTypeTCP:
			l4 = &flow.Layer4{
				Protocol: &flow.Layer4_TCP{
					TCP: &flow.TCP{
						SourcePort:      uint32(packet.TCP.SrcPort),
						DestinationPort: uint32(packet.TCP.DstPort),
						Flags: &flow.TCPFlags{
							FIN: packet.TCP.FIN, SYN: packet.TCP.SYN, RST: packet.TCP.RST,
							PSH: packet.TCP.PSH, ACK: packet.TCP.ACK, URG: packet.TCP.URG,
							ECE: packet.TCP.ECE, CWR: packet.TCP.CWR, NS: packet.TCP.NS,
						},
					},
				},
			}
			sourcePort = uint32(packet.TCP.SrcPort)
			destinationPort = uint32(packet.TCP.DstPort)
			// TODO: TCP Summary
		case layers.LayerTypeUDP:
			l4 = &flow.Layer4{
				Protocol: &flow.Layer4_UDP{
					UDP: &flow.UDP{
						SourcePort:      uint32(packet.UDP.SrcPort),
						DestinationPort: uint32(packet.UDP.DstPort),
					},
				},
			}
			sourcePort = uint32(packet.UDP.SrcPort)
			destinationPort = uint32(packet.UDP.DstPort)
		}
	}

	return
}

func getVerdict(dn *monitor.DropNotify, tn *monitor.TraceNotify, pn *monitor.PolicyVerdictNotify) flow.Verdict {
	switch {
	case dn != nil:
		return flow.Verdict_DROPPED
	case tn != nil:
		return flow.Verdict_FORWARDED
	case pn != nil:
		if pn.Verdict < 0 {
			return flow.Verdict_DROPPED
		}
		if pn.Verdict > 0 {
			return flow.Verdict_REDIRECTED
		}
		if pn.IsTrafficAudited() {
			return flow.Verdict_AUDIT
		}
		return flow.Verdict_FORWARDED
	}
	return flow.Verdict_VERDICT_UNKNOWN
}
