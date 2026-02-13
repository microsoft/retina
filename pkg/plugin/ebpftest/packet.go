// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build ebpf && linux

package ebpftest

import (
	"net"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

var (
	// Default MAC addresses for test packets (arbitrary, eBPF programs don't inspect MACs).
	DefaultSrcMAC = net.HardwareAddr{0x00, 0x00, 0x5e, 0x00, 0x53, 0x01}
	DefaultDstMAC = net.HardwareAddr{0x00, 0x00, 0x5e, 0x00, 0x53, 0x02}
)

// TCPPacketOpts configures a test TCP packet.
type TCPPacketOpts struct {
	SrcIP, DstIP     net.IP
	SrcPort, DstPort uint16
	SYN, ACK, FIN    bool
	RST, PSH, URG    bool
	ECE, CWR         bool
	SeqNum, AckNum   uint32
	TSval, TSecr     uint32 // Zero means omit TCP timestamp option.
	PayloadSize      int
}

// BuildTCPPacket constructs a valid Ethernet + IPv4 + TCP packet.
func BuildTCPPacket(opts TCPPacketOpts) []byte {
	eth := &layers.Ethernet{
		SrcMAC:       DefaultSrcMAC,
		DstMAC:       DefaultDstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    opts.SrcIP.To4(),
		DstIP:    opts.DstIP.To4(),
	}

	tcp := &layers.TCP{
		SrcPort: layers.TCPPort(opts.SrcPort),
		DstPort: layers.TCPPort(opts.DstPort),
		SYN:     opts.SYN,
		ACK:     opts.ACK,
		FIN:     opts.FIN,
		RST:     opts.RST,
		PSH:     opts.PSH,
		URG:     opts.URG,
		ECE:     opts.ECE,
		CWR:     opts.CWR,
		Seq:     opts.SeqNum,
		Ack:     opts.AckNum,
		Window:  65535,
	}

	if opts.TSval != 0 || opts.TSecr != 0 {
		tcp.Options = append(tcp.Options, layers.TCPOption{
			OptionType:   layers.TCPOptionKindTimestamps,
			OptionLength: 10,
			OptionData: []byte{
				byte(opts.TSval >> 24), byte(opts.TSval >> 16), byte(opts.TSval >> 8), byte(opts.TSval),
				byte(opts.TSecr >> 24), byte(opts.TSecr >> 16), byte(opts.TSecr >> 8), byte(opts.TSecr),
			},
		})
	}

	tcp.SetNetworkLayerForChecksum(ip)

	payload := make([]byte, opts.PayloadSize)

	buf := gopacket.NewSerializeBuffer()
	serializeOpts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	err := gopacket.SerializeLayers(buf, serializeOpts,
		eth, ip, tcp, gopacket.Payload(payload))
	if err != nil {
		panic("failed to serialize TCP packet: " + err.Error())
	}

	return buf.Bytes()
}

// UDPPacketOpts configures a test UDP packet.
type UDPPacketOpts struct {
	SrcIP, DstIP     net.IP
	SrcPort, DstPort uint16
	PayloadSize      int
}

// BuildUDPPacket constructs a valid Ethernet + IPv4 + UDP packet.
func BuildUDPPacket(opts UDPPacketOpts) []byte {
	eth := &layers.Ethernet{
		SrcMAC:       DefaultSrcMAC,
		DstMAC:       DefaultDstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    opts.SrcIP.To4(),
		DstIP:    opts.DstIP.To4(),
	}

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(opts.SrcPort),
		DstPort: layers.UDPPort(opts.DstPort),
	}

	udp.SetNetworkLayerForChecksum(ip)

	payload := make([]byte, opts.PayloadSize)

	buf := gopacket.NewSerializeBuffer()
	serializeOpts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	err := gopacket.SerializeLayers(buf, serializeOpts,
		eth, ip, udp, gopacket.Payload(payload))
	if err != nil {
		panic("failed to serialize UDP packet: " + err.Error())
	}

	return buf.Bytes()
}

// BuildNonIPPacket constructs an Ethernet frame with the given EtherType and a small payload.
// Useful for testing that eBPF programs correctly skip non-IPv4 traffic.
func BuildNonIPPacket(etherType layers.EthernetType) []byte {
	eth := &layers.Ethernet{
		SrcMAC:       DefaultSrcMAC,
		DstMAC:       DefaultDstMAC,
		EthernetType: etherType,
	}

	buf := gopacket.NewSerializeBuffer()
	serializeOpts := gopacket.SerializeOptions{FixLengths: true}

	// Add enough payload to be a valid-looking frame.
	err := gopacket.SerializeLayers(buf, serializeOpts,
		eth, gopacket.Payload(make([]byte, 46)))
	if err != nil {
		panic("failed to serialize non-IP packet: " + err.Error())
	}

	return buf.Bytes()
}

// BuildRuntPacket returns a byte slice shorter than 14 bytes (Ethernet header size).
func BuildRuntPacket() []byte {
	return []byte{0x00, 0x00, 0x5e, 0x00, 0x53, 0x01, 0x00, 0x00, 0x5e, 0x00}
}

// BuildTruncatedIPPacket returns a valid Ethernet header followed by a truncated IPv4 header.
func BuildTruncatedIPPacket() []byte {
	eth := &layers.Ethernet{
		SrcMAC:       DefaultSrcMAC,
		DstMAC:       DefaultDstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	buf := gopacket.NewSerializeBuffer()
	serializeOpts := gopacket.SerializeOptions{FixLengths: true}

	err := gopacket.SerializeLayers(buf, serializeOpts, eth)
	if err != nil {
		panic("failed to serialize ethernet header: " + err.Error())
	}

	// Append a truncated IPv4 header (only 10 bytes instead of minimum 20).
	result := buf.Bytes()
	result = append(result, []byte{0x45, 0x00, 0x00, 0x28, 0x00, 0x00, 0x00, 0x00, 0x40, 0x06}...)
	return result
}

// BuildICMPPacket constructs a valid Ethernet + IPv4 + ICMP echo request packet.
func BuildICMPPacket(srcIP, dstIP net.IP) []byte {
	eth := &layers.Ethernet{
		SrcMAC:       DefaultSrcMAC,
		DstMAC:       DefaultDstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolICMPv4,
		SrcIP:    srcIP.To4(),
		DstIP:    dstIP.To4(),
	}

	icmp := &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
		Id:       1,
		Seq:      1,
	}

	buf := gopacket.NewSerializeBuffer()
	serializeOpts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	err := gopacket.SerializeLayers(buf, serializeOpts,
		eth, ip, icmp, gopacket.Payload([]byte("ping")))
	if err != nil {
		panic("failed to serialize ICMP packet: " + err.Error())
	}

	return buf.Bytes()
}

// BuildTruncatedTCPPacket constructs an Ethernet + IPv4 header with a truncated TCP header.
func BuildTruncatedTCPPacket(srcIP, dstIP net.IP) []byte {
	eth := &layers.Ethernet{
		SrcMAC:       DefaultSrcMAC,
		DstMAC:       DefaultDstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    srcIP.To4(),
		DstIP:    dstIP.To4(),
	}

	buf := gopacket.NewSerializeBuffer()
	serializeOpts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	err := gopacket.SerializeLayers(buf, serializeOpts, eth, ip)
	if err != nil {
		panic("failed to serialize IP header: " + err.Error())
	}

	// Append only 10 bytes of TCP header (minimum is 20).
	result := buf.Bytes()
	result = append(result, []byte{0x30, 0x39, 0x00, 0x50, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00}...)
	return result
}
