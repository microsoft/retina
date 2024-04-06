package pktmon

// #cgo CFLAGS: -I packetmonitorsupport
// #cgo LDFLAGS: -L packetmonitorsupport
// #cgo LDFLAGS: -lpktmonapi -lws2_32
//
// #include "PacketMonitor.h"
// #include "packetmonitorpacket.h"
// #include "packetmonitorsupportutil.h"
// #include "packetmonitorsupport.h"
// #include "packetmonitorsupport.c"
// #include "packetmonitorpacketparse.c"
import "C"
import (
	"errors"
	"fmt"
	golog "log"
	"unsafe"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
)

var (
	ErrFailedToParseWithGoPacket  error = fmt.Errorf("Failed to parse with gopacket")
	ErrNotSupported               error = fmt.Errorf("Not supported")
	ErrFailedToStartPacketCapture error = fmt.Errorf("Failed to start pktmon packet capture")
	ErrUnknownPacketType          error = fmt.Errorf("Unknown packet type")

	VarDefaultBufferMultiplier = 10

	TruncationSize = 128
)

type WinPktMon struct {
	l *log.ZapLogger
}

func (w *WinPktMon) Initialize() error {
	var UserContext C.PACKETMONITOR_STREAM_EVENT_INFO

	// calling packet capture routine concurrently
	fmt.Println("Starting (go)")
	trunc := C.int(TruncationSize)
	result := C.InitializePacketCapture(unsafe.Pointer(&UserContext), C.int(VarDefaultBufferMultiplier), trunc)
	if result != 0 {
		return fmt.Errorf("Error code %d, %w   ", result, ErrFailedToStartPacketCapture)
	}

	return nil
}

func (w *WinPktMon) GetNextPacket() (*flow.Flow, *utils.RetinaMetadata, gopacket.Packet, error) {
	buffer := make([]byte, 5000)
	var bufferSize C.int = 5000 // Windows LSO MTU size, Pktmon ring buffers size in Pktmon dll is (64 * 4kb)

	// Three memory buffers
	// - Streaming feature descripter buffer
	// - Descripter buffer
	// - actual packet buffer (64 * 4kb)
	var payloadLength C.int = 0
	var StreamMetaData C.PACKETMONITOR_STREAM_METADATA_RETINA
	var PacketHeaderInfo C.PACKETMONITOR_PACKET_HEADER_INFO
	var MissedPacketsWrite C.int = 0 // packets getting missed in the driver
	var MissedPacketsRead C.int = 0  // packets getting missed in the driver

	// Note: if packet header info of nil is passed, then it wont fall back on to C parsing
	C.GetNextPacket((*C.uchar)(unsafe.Pointer(&buffer[0])), bufferSize, &payloadLength, &StreamMetaData, nil, &MissedPacketsWrite, &MissedPacketsRead)

	if int(MissedPacketsRead) > 0 {
		golog.Printf("Missed packets read: %d\n", int(MissedPacketsRead))
	}

	if int(MissedPacketsWrite) > 0 {
		golog.Printf("Missed packets write: %d\n", int(MissedPacketsWrite))
	}

	packet, err := w.parsePacket(buffer, StreamMetaData)
	if err != nil {
		if errors.Is(err, ErrFailedToParseWithGoPacket) {

			// we will hit this if failing to parse with gopacket, and fall back to C parsing.
			// However in the current impliementation, pulling source/dest info via C libs is nontrivial.
			// To go through C parsing, pass the PacketHeaderInfo struct to the above C.GetNextPacket
			// so marking this tombstone as todo and erroring out.
			if PacketHeaderInfo.ParseErrorCode == 0 {
				return nil, nil, nil, fmt.Errorf("failed to parse with gopacket, using C, but address not impl(src port %d, dst port %d, proto :%d)", PacketHeaderInfo.PortLocal, PacketHeaderInfo.PortRemote, PacketHeaderInfo.IpProtocol)

			} else {
				status := PacketHeaderInfo.ParseErrorCode
				return nil, nil, nil, fmt.Errorf("error code %d: %s, %w", PacketHeaderInfo.ParseErrorCode, C.GoString(C.ParsePacketStatusToString(status)), ErrNotSupported)
			}
		} else {
			return nil, nil, nil, fmt.Errorf("failed to parse with gopacket: %w", err)
		}
	}

	// windows timestamp to unix timestamp
	unixTime := w.getUnixTimestamp(StreamMetaData)

	// get src/dst ip, proto
	ip, err := w.parseL4(packet)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse IP layer: %w", err)
	}

	// get src/dst ports from protocol layers
	tcp, udp := &layers.TCP{}, &layers.UDP{}
	srcPort, dstPort := uint32(0), uint32(0)
	if ip.Protocol == layers.IPProtocolTCP {
		tcp, err = w.parseTCP(packet)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse TCP layer: %w", err)
		}
		srcPort = uint32(tcp.SrcPort)
		dstPort = uint32(tcp.DstPort)
	} else if ip.Protocol == layers.IPProtocolUDP {
		udp, err = w.parseUDP(packet)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse UDP layer: %w", err)
		}
		srcPort = uint32(udp.SrcPort)
		dstPort = uint32(udp.DstPort)
	}

	// get verdict, forwarded, dropped, etc
	verdict := w.getVerdict(StreamMetaData)
	if verdict == flow.Verdict_DROPPED {
		fmt.Printf("packet dropped from %s:%d to %s:%d, proto: %d, \t dropreason %s\n", ip.SrcIP, srcPort, ip.DstIP, dstPort, ip.Protocol, metrics.GetDropReason(uint32(StreamMetaData.DropReason)))
	}

	// create the flow using utils
	fl := utils.ToFlow(
		int64(unixTime), // timestamp
		ip.SrcIP,
		ip.DstIP,
		srcPort,
		dstPort,
		uint8(ip.Protocol),
		uint32(StreamMetaData.ComponentId), // observationPoint
		verdict,                            // flow.Verdict
	)

	// add TCP flags now that we have flow
	if ip.Protocol == layers.IPProtocolTCP {
		utils.AddTcpFlagsBool(fl, tcp.SYN, tcp.ACK, tcp.FIN, tcp.RST, tcp.PSH, tcp.URG)
	}

	// add metadata
	meta := &utils.RetinaMetadata{
		Bytes: uint64(payloadLength),
	}

	return fl, meta, packet, nil
}

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

func parseDNS(fl *flow.Flow, metadata *utils.RetinaMetadata, packet gopacket.Packet) error {
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

		fl.Verdict = utils.Verdict_DNS

		utils.AddDNSInfo(fl, metadata, qtype, uint32(dns.ResponseCode), query, []string{qtype}, len(as), as)
	}

	return nil
}
