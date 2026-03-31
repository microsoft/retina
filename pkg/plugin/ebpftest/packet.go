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
	TSval, TSecr     uint32  // Zero means omit TCP timestamp option.
	Payload          []byte  // Raw payload bytes. If nil, PayloadSize zero bytes are used.
	PayloadSize      int     // Ignored when Payload is set.
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

	payload := opts.Payload
	if payload == nil {
		payload = make([]byte, opts.PayloadSize)
	}

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
	Payload          []byte // Raw payload bytes. If nil, PayloadSize zero bytes are used.
	PayloadSize      int    // Ignored when Payload is set.
}

// BuildUDPPacket constructs a valid Ethernet + IP + UDP packet.
// Automatically selects IPv4 or IPv6 based on the source IP address.
func BuildUDPPacket(opts UDPPacketOpts) []byte {
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(opts.SrcPort),
		DstPort: layers.UDPPort(opts.DstPort),
	}

	payload := opts.Payload
	if payload == nil {
		payload = make([]byte, opts.PayloadSize)
	}

	serOpts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	if isIPv6(opts.SrcIP) {
		return serializeUDPv6(opts.SrcIP, opts.DstIP, udp, payload, serOpts)
	}
	return serializeUDPv4(opts.SrcIP, opts.DstIP, udp, payload, serOpts)
}

func serializeUDPv4(srcIP, dstIP net.IP, udp *layers.UDP, payload []byte, serOpts gopacket.SerializeOptions) []byte {
	eth := &layers.Ethernet{SrcMAC: DefaultSrcMAC, DstMAC: DefaultDstMAC, EthernetType: layers.EthernetTypeIPv4}
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, Protocol: layers.IPProtocolUDP, SrcIP: srcIP.To4(), DstIP: dstIP.To4()}
	udp.SetNetworkLayerForChecksum(ip)
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, serOpts, eth, ip, udp, gopacket.Payload(payload)); err != nil {
		panic("failed to serialize UDPv4 packet: " + err.Error())
	}
	return buf.Bytes()
}

func serializeUDPv6(srcIP, dstIP net.IP, udp *layers.UDP, payload []byte, serOpts gopacket.SerializeOptions) []byte {
	eth := &layers.Ethernet{SrcMAC: DefaultSrcMAC, DstMAC: DefaultDstMAC, EthernetType: layers.EthernetTypeIPv6}
	ip := &layers.IPv6{Version: 6, HopLimit: 64, NextHeader: layers.IPProtocolUDP, SrcIP: srcIP.To16(), DstIP: dstIP.To16()}
	udp.SetNetworkLayerForChecksum(ip)
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, serOpts, eth, ip, udp, gopacket.Payload(payload)); err != nil {
		panic("failed to serialize UDPv6 packet: " + err.Error())
	}
	return buf.Bytes()
}

// isIPv6 returns true if ip is an IPv6 address (not IPv4-mapped).
func isIPv6(ip net.IP) bool {
	return ip.To4() == nil
}

// DNSQueryOpts configures a test DNS query packet (Ethernet + IP + UDP + DNS).
type DNSQueryOpts struct {
	SrcIP, DstIP     net.IP
	SrcPort, DstPort uint16
	ID               uint16
	Name             string
	QType            layers.DNSType
}

// BuildDNSQueryPacket constructs a complete Ethernet + IP + UDP + DNS query packet.
// Automatically selects IPv4 or IPv6 based on the source IP address.
func BuildDNSQueryPacket(opts DNSQueryOpts) []byte {
	dns := &layers.DNS{
		ID:      opts.ID,
		RD:      true,
		QDCount: 1,
		Questions: []layers.DNSQuestion{{
			Name:  []byte(opts.Name),
			Type:  opts.QType,
			Class: layers.DNSClassIN,
		}},
	}
	return buildDNSPacket(opts.SrcIP, opts.DstIP, opts.SrcPort, opts.DstPort, dns)
}

// DNSResponseOpts configures a test DNS response packet (Ethernet + IP + UDP + DNS).
type DNSResponseOpts struct {
	SrcIP, DstIP     net.IP
	SrcPort, DstPort uint16
	ID               uint16
	Name             string
	QType            layers.DNSType
	RCode            layers.DNSResponseCode
	Answers          []net.IP // A/AAAA record answers.
}

// BuildDNSResponsePacket constructs a complete Ethernet + IP + UDP + DNS response packet.
// Automatically selects IPv4 or IPv6 based on the source IP address.
func BuildDNSResponsePacket(opts DNSResponseOpts) []byte {
	dns := &layers.DNS{
		ID:           opts.ID,
		QR:           true,
		RD:           true,
		ResponseCode: opts.RCode,
		QDCount:      1,
		Questions: []layers.DNSQuestion{{
			Name:  []byte(opts.Name),
			Type:  opts.QType,
			Class: layers.DNSClassIN,
		}},
	}
	for _, ip := range opts.Answers {
		rr := layers.DNSResourceRecord{
			Name:  []byte(opts.Name),
			Type:  opts.QType,
			Class: layers.DNSClassIN,
			TTL:   60,
			IP:    ip,
		}
		dns.Answers = append(dns.Answers, rr)
	}
	dns.ANCount = uint16(len(dns.Answers))
	return buildDNSPacket(opts.SrcIP, opts.DstIP, opts.SrcPort, opts.DstPort, dns)
}

// serializeDNS returns the raw bytes of a DNS layer.
func serializeDNS(dns *layers.DNS) []byte {
	buf := gopacket.NewSerializeBuffer()
	if err := dns.SerializeTo(buf, gopacket.SerializeOptions{FixLengths: true}); err != nil {
		panic("failed to serialize DNS: " + err.Error())
	}
	return buf.Bytes()
}

// buildDNSPacket serializes a DNS layer inside an Ethernet + IP + UDP frame.
func buildDNSPacket(srcIP, dstIP net.IP, srcPort, dstPort uint16, dns *layers.DNS) []byte {
	return BuildUDPPacket(UDPPacketOpts{
		SrcIP: srcIP, DstIP: dstIP, SrcPort: srcPort, DstPort: dstPort,
		Payload: serializeDNS(dns),
	})
}

// BuildDNSTCPQueryPacket constructs an Ethernet + IPv4 + TCP packet carrying a
// DNS query. DNS over TCP prepends a 2-byte length field before the message.
func BuildDNSTCPQueryPacket(opts DNSQueryOpts) []byte {
	dnsBytes := serializeDNS(&layers.DNS{
		ID: opts.ID, RD: true, QDCount: 1,
		Questions: []layers.DNSQuestion{{
			Name: []byte(opts.Name), Type: opts.QType, Class: layers.DNSClassIN,
		}},
	})
	tcpPayload := make([]byte, 2+len(dnsBytes))
	tcpPayload[0] = byte(len(dnsBytes) >> 8)
	tcpPayload[1] = byte(len(dnsBytes))
	copy(tcpPayload[2:], dnsBytes)

	return BuildTCPPacket(TCPPacketOpts{
		SrcIP: opts.SrcIP, DstIP: opts.DstIP,
		SrcPort: opts.SrcPort, DstPort: opts.DstPort,
		PSH: true, ACK: true,
		Payload: tcpPayload,
	})
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
