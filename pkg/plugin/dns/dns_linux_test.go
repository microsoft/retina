// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package dns

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"unsafe"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf/perf"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMain(m *testing.M) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics(slog.Default())
	os.Exit(m.Run())
}

func TestNew(t *testing.T) {
	cfg := &config.Config{EnablePodLevel: true}
	d := New(cfg)
	require.NotNil(t, d)
	assert.Equal(t, name, d.Name())
}

func TestStop(t *testing.T) {
	d := &dns{
		cfg: &config.Config{EnablePodLevel: true},
		l:   log.Logger().Named(name),
	}
	// Should not panic when not running.
	require.NoError(t, d.Stop())
}

func TestSetupChannel(t *testing.T) {
	d := &dns{
		cfg: &config.Config{EnablePodLevel: true},
		l:   log.Logger().Named(name),
	}
	ch := make(chan *v1.Event, 10)
	require.NoError(t, d.SetupChannel(ch))
	assert.Equal(t, ch, d.externalChannel)
}

func TestGenerate(t *testing.T) {
	d := &dns{cfg: &config.Config{}, l: log.Logger().Named(name)}
	require.NoError(t, d.Generate(context.Background()))
}

// buildTestRecord creates a synthetic perf.Record containing a dns_event struct
// followed by raw packet bytes, mimicking what the BPF program produces.
// The struct is reinterpreted as raw bytes via unsafe — the same layout the
// BPF perf ring uses, so handleDNSEvent's unsafe cast works correctly.
func buildTestRecord(event dnsDnsEvent, packetData []byte) perf.Record {
	eventSize := int(unsafe.Sizeof(event))
	eventBytes := unsafe.Slice((*byte)(unsafe.Pointer(&event)), eventSize) //nolint:gosec // test-only
	buf := make([]byte, eventSize+len(packetData))
	copy(buf, eventBytes)
	copy(buf[eventSize:], packetData)
	return perf.Record{RawSample: buf}
}

// TestHandleDNSEvent_RequestCounter verifies the request counter increments
// for QR=0 events when pod-level is disabled (only counters, no flow).
func TestHandleDNSEvent_RequestCounter(t *testing.T) {
	d := &dns{
		cfg: &config.Config{EnablePodLevel: false},
		l:   log.Logger().Named(name),
	}
	before := testutil.ToFloat64(metrics.DNSRequestCounter.WithLabelValues())
	record := buildTestRecord(dnsDnsEvent{Timestamp: 1000, Qr: 0}, nil)
	d.handleDNSEvent(record)
	after := testutil.ToFloat64(metrics.DNSRequestCounter.WithLabelValues())
	assert.InDelta(t, before+1, after, 0, "request counter should increment by 1")
}

// TestHandleDNSEvent_ResponseCounter verifies the response counter increments
// for QR=1 events.
func TestHandleDNSEvent_ResponseCounter(t *testing.T) {
	d := &dns{
		cfg: &config.Config{EnablePodLevel: false},
		l:   log.Logger().Named(name),
	}
	before := testutil.ToFloat64(metrics.DNSResponseCounter.WithLabelValues())
	record := buildTestRecord(dnsDnsEvent{Timestamp: 2000, Qr: 1, Rcode: 3}, nil)
	d.handleDNSEvent(record)
	after := testutil.ToFloat64(metrics.DNSResponseCounter.WithLabelValues())
	assert.InDelta(t, before+1, after, 0, "response counter should increment by 1")
}

// TestHandleDNSEvent_TooShortRecord verifies a record shorter than the event
// struct is silently skipped (no counter change).
func TestHandleDNSEvent_TooShortRecord(t *testing.T) {
	d := &dns{
		cfg: &config.Config{EnablePodLevel: true},
		l:   log.Logger().Named(name),
	}
	reqBefore := testutil.ToFloat64(metrics.DNSRequestCounter.WithLabelValues())
	respBefore := testutil.ToFloat64(metrics.DNSResponseCounter.WithLabelValues())
	record := perf.Record{RawSample: []byte{0x01, 0x02, 0x03}}
	d.handleDNSEvent(record)
	assert.InDelta(t, reqBefore, testutil.ToFloat64(metrics.DNSRequestCounter.WithLabelValues()), 0)
	assert.InDelta(t, respBefore, testutil.ToFloat64(metrics.DNSResponseCounter.WithLabelValues()), 0)
}

// TestHandleDNSEvent_PktTypeIgnored verifies that pkt_type does not affect
// flow emission — direction is derived from QR, not pkt_type. Even
// PACKET_OUTGOING (normally filtered by BPF) produces a flow if it somehow
// reaches userspace.
func TestHandleDNSEvent_PktTypeIgnored(t *testing.T) {
	ch := make(chan *v1.Event, 10)
	d := &dns{
		cfg:             &config.Config{EnablePodLevel: true},
		l:               log.Logger().Named(name),
		externalChannel: ch,
	}
	record := buildTestRecord(dnsDnsEvent{
		Timestamp: 1000, Af: 4, PktType: 4, // PACKET_OUTGOING (filtered by BPF, not by Go)
		SrcIp: 0x0100000A, DstIp: 0x0200000A,
	}, nil)
	d.handleDNSEvent(record)
	assert.Len(t, ch, 1, "pkt_type should not affect flow emission; direction comes from QR")
}

// TestHandleDNSEvent_UnknownAF verifies packets with an unknown address family
// are dropped when pod-level is enabled.
func TestHandleDNSEvent_UnknownAF(t *testing.T) {
	ch := make(chan *v1.Event, 10)
	d := &dns{
		cfg:             &config.Config{EnablePodLevel: true},
		l:               log.Logger().Named(name),
		externalChannel: ch,
	}
	record := buildTestRecord(dnsDnsEvent{
		Timestamp: 1000, Af: 99, PktType: 0,
	}, nil)
	d.handleDNSEvent(record)
	assert.Empty(t, ch, "unknown AF should not emit a flow")
}

// TestParseDNSPayload_Query verifies query name and type extraction.
func TestParseDNSPayload_Query(t *testing.T) {
	d := &dns{l: log.Logger().Named(name)}

	payload := buildMinimalDNSQuery("example.com", 1)
	dnsName, addresses, qtype := d.parseDNSPayload(payload, false)

	assert.Equal(t, "example.com.", dnsName)
	assert.Empty(t, addresses)
	assert.Equal(t, uint16(1), uint16(qtype))
}

// TestParseDNSPayload_Response verifies response parsing with an A record.
func TestParseDNSPayload_Response(t *testing.T) {
	d := &dns{l: log.Logger().Named(name)}

	payload := buildMinimalDNSResponse("test.local", 1, net.ParseIP("1.2.3.4"))
	dnsName, addresses, qtype := d.parseDNSPayload(payload, true)

	assert.Equal(t, "test.local.", dnsName)
	assert.Equal(t, uint16(1), uint16(qtype))
	require.Len(t, addresses, 1)
	assert.Equal(t, "1.2.3.4", addresses[0])
}

// TestParseDNSPayload_TooShort verifies payloads under 12 bytes return empty.
func TestParseDNSPayload_TooShort(t *testing.T) {
	d := &dns{l: log.Logger().Named(name)}

	dnsName, addresses, qtype := d.parseDNSPayload([]byte{0x00, 0x01}, false)

	assert.Empty(t, dnsName)
	assert.Nil(t, addresses)
	assert.Equal(t, uint16(0), uint16(qtype))
}

// TestParseDNSPayload_Malformed verifies malformed DNS payloads don't panic.
func TestParseDNSPayload_Malformed(t *testing.T) {
	d := &dns{l: log.Logger().Named(name)}

	// 12-byte header with QDCOUNT=1 but no question data.
	payload := make([]byte, 20)
	payload[4] = 0x00
	payload[5] = 0x01
	assert.NotPanics(t, func() {
		d.parseDNSPayload(payload, false)
	})
}

// TestHandleDNSEvent_WithPacketData verifies handleDNSEvent processes a
// complete event with attached packet data, writes to the enricher, and
// emits a flow to the external channel with correct DNS info.
func TestHandleDNSEvent_WithPacketData(t *testing.T) {
	ctrl := gomock.NewController(t)

	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck
	menricher.EXPECT().Write(gomock.Any()).MinTimes(1)

	ch := make(chan *v1.Event, 10)
	d := &dns{
		cfg:             &config.Config{EnablePodLevel: true},
		l:               log.Logger().Named(name),
		enricher:        menricher,
		externalChannel: ch,
	}

	dnsPayload := buildMinimalDNSQuery("k8s.io", 1)
	// Build a minimal packet: Ethernet(14) + IP(20) + UDP(8) + DNS.
	// dns_off = 42 points to the DNS payload within the packet.
	pktData := make([]byte, 42+len(dnsPayload))
	copy(pktData[42:], dnsPayload)

	record := buildTestRecord(dnsDnsEvent{
		Timestamp: 5000,
		Af:        4,
		Proto:     17,
		PktType:   0, // PACKET_HOST
		Qr:        0,
		Qtype:     1,
		SrcPort:   12345,
		DstPort:   53,
		DnsOff:    42,
		SrcIp:     0x0100000A, // 10.0.0.1
		DstIp:     0x0200000A, // 10.0.0.2
	}, pktData)

	d.handleDNSEvent(record)

	require.Len(t, ch, 1, "expected one flow on the external channel")
	ev := <-ch
	fl := ev.GetFlow()
	require.NotNil(t, fl)
	assert.NotNil(t, fl.GetL4().GetUDP())
	assert.Equal(t, uint32(53), fl.GetL4().GetUDP().GetDestinationPort())
	assert.NotNil(t, fl.GetExtensions())
}

// --- DNS payload helpers ---

func buildMinimalDNSQuery(name string, qtype uint16) []byte {
	buf := gopacket.NewSerializeBuffer()
	dns := &layers.DNS{
		ID: 1, RD: true, QDCount: 1,
		Questions: []layers.DNSQuestion{{
			Name: []byte(name), Type: layers.DNSType(qtype), Class: layers.DNSClassIN,
		}},
	}
	if err := dns.SerializeTo(buf, gopacket.SerializeOptions{FixLengths: true}); err != nil {
		panic("failed to serialize DNS query: " + err.Error())
	}
	return buf.Bytes()
}

func buildMinimalDNSResponse(name string, qtype uint16, answerIP net.IP) []byte {
	buf := gopacket.NewSerializeBuffer()
	dns := &layers.DNS{
		ID: 1, QR: true, RD: true, QDCount: 1, ANCount: 1,
		Questions: []layers.DNSQuestion{{
			Name: []byte(name), Type: layers.DNSType(qtype), Class: layers.DNSClassIN,
		}},
		Answers: []layers.DNSResourceRecord{{
			Name: []byte(name), Type: layers.DNSType(qtype), Class: layers.DNSClassIN,
			TTL: 60, IP: answerIP,
		}},
	}
	if err := dns.SerializeTo(buf, gopacket.SerializeOptions{FixLengths: true}); err != nil {
		panic("failed to serialize DNS response: " + err.Error())
	}
	return buf.Bytes()
}
