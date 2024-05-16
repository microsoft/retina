package pktmon

import (
	"fmt"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

func (w *WinPktMon) getUnixTimestamp(StreamMetaData C.PACKETMONITOR_STREAM_METADATA_RETINA) int64 {
	// use C conversion to pull Windows timestamp
	var timestampint C.longlong
	C.LargeIntegerToInt(StreamMetaData.TimeStamp, &timestampint)
	timestamp := int64(timestampint)

	// convert from windows to unix time
	var epochDifference int64 = 116444736000000000
	return (timestamp - epochDifference) / 10000000
}

func (w *WinPktMon) getVerdict(StreamMetaData C.PACKETMONITOR_STREAM_METADATA_RETINA) flow.Verdict {
	if StreamMetaData.DropReason != 0 {
		return flow.Verdict_DROPPED
	}
	return flow.Verdict_FORWARDED
}

func (w *WinPktMon) parseL4(packet gopacket.Packet) (*layers.IPv4, error) {
	ip := &layers.IPv4{}
	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip, _ = ipLayer.(*layers.IPv4)
	} else {
		return nil, fmt.Errorf("Failed to parse IP layer %w", ErrFailedToParseWithGoPacket)
	}
	return ip, nil
}

func (w *WinPktMon) parseTCP(packet gopacket.Packet) (*layers.TCP, error) {
	tcp := &layers.TCP{}
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ = tcpLayer.(*layers.TCP)
	} else {
		return nil, fmt.Errorf("Failed to parse TCP layer %w", ErrFailedToParseWithGoPacket)
	}
	return tcp, nil
}

func (w *WinPktMon) parseUDP(packet gopacket.Packet) (*layers.UDP, error) {
	udp := &layers.UDP{}
	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ = udpLayer.(*layers.UDP)
	} else {
		return nil, fmt.Errorf("Failed to parse UDP layer %w", ErrFailedToParseWithGoPacket)
	}
	return udp, nil
}

func (w *WinPktMon) parsePacket(buffer []byte, StreamMetaData C.PACKETMONITOR_STREAM_METADATA_RETINA) (gopacket.Packet, error) {
	var packet gopacket.Packet
	// Ethernet
	if StreamMetaData.PacketType == 1 {
		packet = gopacket.NewPacket(buffer, layers.LayerTypeEthernet, gopacket.NoCopy)

		// IPv4
	} else if StreamMetaData.PacketType == 3 {
		packet = gopacket.NewPacket(buffer, layers.LayerTypeIPv4, gopacket.NoCopy)
	} else {
		return nil, ErrUnknownPacketType
	}
	return packet, nil
}

func (w *WinPktMon) ParseDNS(fl *flow.Flow, metadata *utils.RetinaMetadata, packet gopacket.Packet) error {
	dns := &layers.DNS{}
	if dnsLayer := packet.Layer(layers.LayerTypeDNS); dnsLayer != nil {
		dns, _ = dnsLayer.(*layers.DNS)
	} else {
		return nil
	}

	if dns != nil {
		//fmt.Printf("qType %d\n", packet.dns.OpCode)
		var qtype string
		switch dns.OpCode {
		case layers.DNSOpCodeQuery:
			qtype = "Q"
		case layers.DNSOpCodeStatus:
			qtype = "R"
		default:
			qtype = "U"
		}

		var as, qs []string
		for _, a := range dns.Answers {
			if a.IP != nil {
				as = append(as, a.IP.String())
			}
		}
		for _, q := range dns.Questions {
			qs = append(qs, string(q.Name))
		}

		var query string
		if len(dns.Questions) > 0 {
			query = string(dns.Questions[0].Name[:])
		}
		w.l.Debug("DNS packet", zap.String("query", query), zap.String("qtype", qtype), zap.String("answers", fmt.Sprintf("%v", as)))
		fl.Verdict = utils.Verdict_DNS
		utils.AddDNSInfo(fl, metadata, qtype, uint32(dns.ResponseCode), query, []string{qtype}, len(as), as)
	}

	return nil
}
