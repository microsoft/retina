// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build ebpf && linux

// Tests for the DNS BPF socket filter program.
//
// These load the compiled BPF program and run it against crafted packets
// using BPF_PROG_TEST_RUN, then read the per-CPU scratch map to verify
// the kernel-side parsing logic.
//
// Requires: root (or CAP_BPF+CAP_NET_ADMIN), Linux kernel 5.10+.
// Run: sudo go test -tags=ebpf -v -count=1 ./pkg/plugin/dns/...

package dns

import (
	"net"
	"testing"

	"github.com/gopacket/gopacket/layers"
	"github.com/microsoft/retina/pkg/plugin/ebpftest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadTestObjects loads the DNS BPF program and maps for testing.
func loadTestObjects(t *testing.T) *dnsObjects {
	t.Helper()
	ebpftest.RequirePrivileged(t)

	spec, err := loadDns()
	require.NoError(t, err)
	ebpftest.RemoveMapPinning(spec)

	var objs dnsObjects
	err = spec.LoadAndAssign(&objs, nil)
	require.NoError(t, err)
	t.Cleanup(func() { objs.Close() })
	return &objs
}

// --- Tests ---

// TestDNSQueryFields sends a standard A record query and verifies all event
// fields: address family, protocol, ports, DNS ID, QR, QTYPE, IPs, and dns_off.
func TestDNSQueryFields(t *testing.T) {
	objs := loadTestObjects(t)

	srcIP := net.ParseIP("10.0.0.1")
	dstIP := net.ParseIP("10.0.0.2")
	pkt := ebpftest.BuildDNSQueryPacket(ebpftest.DNSQueryOpts{
		SrcIP: srcIP, DstIP: dstIP,
		SrcPort: 12345, DstPort: 53,
		ID: 0x1234, Name: "kubernetes.default.svc", QType: layers.DNSTypeA,
	})

	ret := ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)
	assert.Equal(t, uint32(0), ret)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok, "expected DNS event")

	assert.Equal(t, uint8(4), event.Af, "address family")
	assert.Equal(t, uint8(17), event.Proto, "protocol = UDP")
	assert.Equal(t, uint16(12345), event.SrcPort)
	assert.Equal(t, uint16(53), event.DstPort)
	assert.Equal(t, uint16(0x1234), event.Id, "DNS ID")
	assert.Equal(t, uint8(0), event.Qr, "should be query (QR=0)")
	assert.Equal(t, uint16(1), event.Qtype, "QTYPE = A")
	assert.Equal(t, uint16(0), event.Ancount, "query has no answers")
	assert.NotZero(t, event.Timestamp)
	assert.Equal(t, ebpftest.IPToNative("10.0.0.1"), event.SrcIp)
	assert.Equal(t, ebpftest.IPToNative("10.0.0.2"), event.DstIp)
	// dns_off = ETH(14) + IP(20) + UDP(8) = 42
	assert.Equal(t, uint16(42), event.DnsOff)
}

// TestDNSResponseFields sends a NOERROR response with one A record answer and
// verifies QR=1, RCODE, answer count, and QTYPE extraction.
func TestDNSResponseFields(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildDNSResponsePacket(ebpftest.DNSResponseOpts{
		SrcIP: net.ParseIP("8.8.8.8"), DstIP: net.ParseIP("10.0.0.1"),
		SrcPort: 53, DstPort: 40000,
		ID: 0xABCD, Name: "example.com", QType: layers.DNSTypeA,
		RCode:   layers.DNSResponseCodeNoErr,
		Answers: []net.IP{net.ParseIP("93.184.216.34")},
	})

	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok)

	assert.Equal(t, uint8(1), event.Qr, "should be response (QR=1)")
	assert.Equal(t, uint8(0), event.Rcode, "RCODE = NOERROR")
	assert.Equal(t, uint16(0xABCD), event.Id)
	assert.Equal(t, uint16(1), event.Ancount)
	assert.Equal(t, uint16(1), event.Qtype, "QTYPE = A")
}

// TestDNSResponseNXDOMAIN verifies RCODE=3 (NXDOMAIN) is correctly extracted
// from a negative response with zero answers.
func TestDNSResponseNXDOMAIN(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildDNSResponsePacket(ebpftest.DNSResponseOpts{
		SrcIP: net.ParseIP("8.8.8.8"), DstIP: net.ParseIP("10.0.0.1"),
		SrcPort: 53, DstPort: 40000,
		ID: 0x5678, Name: "nxdomain.test", QType: layers.DNSTypeA,
		RCode: layers.DNSResponseCodeNXDomain,
	})

	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok)

	assert.Equal(t, uint8(1), event.Qr)
	assert.Equal(t, uint8(3), event.Rcode, "RCODE = NXDOMAIN")
	assert.Equal(t, uint16(0), event.Ancount)
}

// TestAAAAQuery verifies QTYPE extraction for AAAA (28) queries.
func TestAAAAQuery(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildDNSQueryPacket(ebpftest.DNSQueryOpts{
		SrcIP: net.ParseIP("10.0.0.5"), DstIP: net.ParseIP("10.0.0.10"),
		SrcPort: 55555, DstPort: 53,
		ID: 0x9999, Name: "google.com", QType: layers.DNSTypeAAAA,
	})

	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok)

	assert.Equal(t, uint16(28), event.Qtype, "QTYPE = AAAA")
	assert.Equal(t, uint8(0), event.Qr)
}

// TestIPv6DNSQuery verifies parsing of a DNS query over IPv6, including
// address family, IPv6 address extraction, and QTYPE.
func TestIPv6DNSQuery(t *testing.T) {
	objs := loadTestObjects(t)

	srcIP := net.ParseIP("fd00::1")
	dstIP := net.ParseIP("fd00::2")
	pkt := ebpftest.BuildDNSQueryPacket(ebpftest.DNSQueryOpts{
		SrcIP: srcIP, DstIP: dstIP,
		SrcPort: 44444, DstPort: 53,
		ID: 0xBEEF, Name: "v6.example.com", QType: layers.DNSTypeAAAA,
	})

	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok, "expected DNS event for IPv6")

	assert.Equal(t, uint8(6), event.Af, "address family = IPv6")
	assert.Equal(t, uint8(17), event.Proto, "protocol = UDP")
	assert.Equal(t, uint16(44444), event.SrcPort)
	assert.Equal(t, uint16(53), event.DstPort)
	assert.Equal(t, uint16(0xBEEF), event.Id)
	assert.Equal(t, uint8(0), event.Qr, "should be query")
	assert.Equal(t, uint16(28), event.Qtype, "QTYPE = AAAA")
	// Verify IPv6 addresses (stored as raw 16-byte network order).
	assert.Equal(t, srcIP.To16(), net.IP(event.SrcIp6[:]))
	assert.Equal(t, dstIP.To16(), net.IP(event.DstIp6[:]))
	// IPv4 fields should be zero for IPv6 packets.
	assert.Equal(t, uint32(0), event.SrcIp)
	assert.Equal(t, uint32(0), event.DstIp)
	// dns_off = ETH(14) + IPv6(40) + UDP(8) = 62
	assert.Equal(t, uint16(62), event.DnsOff)
}

// TestIPv6DNSResponse verifies parsing of a DNS response over IPv6.
func TestIPv6DNSResponse(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildDNSResponsePacket(ebpftest.DNSResponseOpts{
		SrcIP: net.ParseIP("2001:4860:4860::8888"), DstIP: net.ParseIP("fd00::1"),
		SrcPort: 53, DstPort: 50000,
		ID: 0xCAFE, Name: "ipv6.example.com", QType: layers.DNSTypeA,
		RCode:   layers.DNSResponseCodeNoErr,
		Answers: []net.IP{net.ParseIP("93.184.216.34")},
	})

	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok)

	assert.Equal(t, uint8(6), event.Af, "address family = IPv6")
	assert.Equal(t, uint8(1), event.Qr, "should be response")
	assert.Equal(t, uint8(0), event.Rcode, "RCODE = NOERROR")
	assert.Equal(t, uint16(0xCAFE), event.Id)
	assert.Equal(t, uint16(1), event.Ancount)
	assert.Equal(t, uint16(1), event.Qtype, "QTYPE = A")
}

// TestMDNSPort verifies that mDNS traffic on port 5353 is recognized as DNS.
func TestMDNSPort(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildDNSQueryPacket(ebpftest.DNSQueryOpts{
		SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("224.0.0.251"),
		SrcPort: 5353, DstPort: 5353,
		ID: 0x1111, Name: "_http._tcp.local", QType: layers.DNSTypePTR,
	})

	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok)

	assert.Equal(t, uint16(5353), event.SrcPort)
	assert.Equal(t, uint16(5353), event.DstPort)
	assert.Equal(t, uint16(12), event.Qtype, "QTYPE = PTR")
}

// TestMultiLabelDomain sends a query with a 15-label domain name to exercise
// the BPF QNAME walking loop and verify QTYPE is correctly extracted.
func TestMultiLabelDomain(t *testing.T) {
	objs := loadTestObjects(t)

	name := "a.b.c.d.e.f.g.h.i.j.kubernetes.default.svc.cluster.local"
	pkt := ebpftest.BuildDNSQueryPacket(ebpftest.DNSQueryOpts{
		SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2"),
		SrcPort: 12345, DstPort: 53,
		ID: 0x4242, Name: name, QType: layers.DNSTypeSRV,
	})

	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok)

	assert.Equal(t, uint16(33), event.Qtype, "QTYPE = SRV")
}

// TestTCPDNS verifies DNS-over-TCP parsing: the BPF program must skip the TCP
// header (using the data offset field) and the 2-byte DNS length prefix.
func TestTCPDNS(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildDNSTCPQueryPacket(ebpftest.DNSQueryOpts{
		SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2"),
		SrcPort: 12345, DstPort: 53,
		ID: 0x7777, Name: "tcp.example.com", QType: layers.DNSTypeA,
	})

	ret := ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)
	assert.Equal(t, uint32(0), ret)

	event, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	require.True(t, ok, "expected DNS event for TCP")

	assert.Equal(t, uint8(6), event.Proto, "protocol = TCP")
	assert.Equal(t, uint16(12345), event.SrcPort)
	assert.Equal(t, uint16(53), event.DstPort)
	assert.Equal(t, uint16(0x7777), event.Id)
	assert.Equal(t, uint16(1), event.Qtype, "QTYPE = A")
	// dns_off = ETH(14) + IP(20) + TCP(20) + len_prefix(2) = 56
	assert.Equal(t, uint16(56), event.DnsOff)
}

// TestTCPDNSEdgeCases verifies the BPF program handles TCP edge cases without
// crashing: a SYN to port 53 with no payload, and a segment with only the
// 2-byte length prefix but no DNS data. In both cases the program may write
// a partial event to the scratch map but does NOT call bpf_perf_event_output,
// so userspace never sees it.
func TestTCPDNSEdgeCases(t *testing.T) {
	t.Run("SYN no payload", func(t *testing.T) {
		objs := loadTestObjects(t)
		pkt := ebpftest.BuildTCPPacket(ebpftest.TCPPacketOpts{
			SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2"),
			SrcPort: 12345, DstPort: 53,
			SYN: true,
		})
		ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)
	})

	t.Run("length prefix only", func(t *testing.T) {
		objs := loadTestObjects(t)
		pkt := ebpftest.BuildTCPPacket(ebpftest.TCPPacketOpts{
			SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2"),
			SrcPort: 12345, DstPort: 53,
			PSH: true, ACK: true,
			Payload: []byte{0x00, 0x20}, // claims 32 bytes but has none
		})
		ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)
	})
}

// TestNonDNSPort_NoEvent verifies that UDP packets to non-DNS ports (neither
// 53 nor 5353) are silently dropped without writing an event.
func TestNonDNSPort_NoEvent(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildUDPPacket(ebpftest.UDPPacketOpts{
		SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2"),
		SrcPort: 12345, DstPort: 8080,
		PayloadSize: 20,
	})
	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	_, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	assert.False(t, ok, "non-DNS port should not produce an event")
}

// TestICMP_NoEvent verifies that non-TCP/UDP protocols are filtered out.
func TestICMP_NoEvent(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildICMPPacket(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2"))
	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	_, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	assert.False(t, ok, "ICMP should not produce a DNS event")
}

// TestNonIPv4_NoEvent verifies that non-IP EtherTypes (ARP) are filtered out.
func TestNonIPv4_NoEvent(t *testing.T) {
	objs := loadTestObjects(t)

	pkt := ebpftest.BuildNonIPPacket(layers.EthernetTypeARP)
	ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

	_, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
	assert.False(t, ok, "ARP should not produce a DNS event")
}

// TestMalformedPackets verifies the BPF program handles edge cases without
// crashing: runt frames, truncated IP headers, and payloads too short for
// a DNS header.
func TestMalformedPackets(t *testing.T) {
	t.Run("runt packet", func(t *testing.T) {
		objs := loadTestObjects(t)
		pkt := ebpftest.BuildRuntPacket()
		padded := append(make([]byte, 14), pkt...) //nolint:gocritic // intentional prepend
		// Runt packets may be rejected by the kernel (EINVAL). Verify no panic.
		_, _, err := objs.RetinaDnsFilter.Test(padded)
		_ = err
	})

	t.Run("truncated IP", func(t *testing.T) {
		objs := loadTestObjects(t)
		pkt := ebpftest.BuildTruncatedIPPacket()
		ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)

		_, ok := ebpftest.ReadPerCPUMap[dnsDnsEvent](t, objs.TmpDnsEvents, 0)
		assert.False(t, ok, "truncated IP should not produce an event")
	})

	t.Run("UDP too short for DNS header", func(t *testing.T) {
		objs := loadTestObjects(t)
		// DNS port but payload < 12 bytes (DNS header size).
		// BPF program writes partial event (timestamp) to scratch map but
		// does NOT call bpf_perf_event_output, so userspace never sees it.
		// Just verify no crash.
		pkt := ebpftest.BuildUDPPacket(ebpftest.UDPPacketOpts{
			SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2"),
			SrcPort: 12345, DstPort: 53,
			Payload: []byte{0x00, 0x01},
		})
		ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, pkt)
	})
}

// TestReturnValue verifies the socket filter always returns 0 regardless of
// packet type. The DNS plugin captures events via perf buffer, not the
// socket filter return value.
func TestReturnValue(t *testing.T) {
	objs := loadTestObjects(t)

	tests := []struct {
		name string
		pkt  []byte
	}{
		{
			"DNS query",
			ebpftest.BuildDNSQueryPacket(ebpftest.DNSQueryOpts{
				SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2"),
				SrcPort: 12345, DstPort: 53,
				ID: 1, Name: "test.com", QType: layers.DNSTypeA,
			}),
		},
		{
			"non-DNS UDP",
			ebpftest.BuildUDPPacket(ebpftest.UDPPacketOpts{
				SrcIP: net.ParseIP("10.0.0.1"), DstIP: net.ParseIP("10.0.0.2"),
				SrcPort: 1000, DstPort: 8080, PayloadSize: 20,
			}),
		},
		{
			"ICMP",
			ebpftest.BuildICMPPacket(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret := ebpftest.RunSocketFilter(t, objs.RetinaDnsFilter, tt.pkt)
			assert.Equal(t, uint32(0), ret, "socket filter should always return 0")
		})
	}
}
