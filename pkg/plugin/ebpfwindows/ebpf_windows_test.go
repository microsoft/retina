// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package ebpfwindows

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/byteorder"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	observerTypes "github.com/cilium/cilium/pkg/hubble/observer/types"
	hubbleerrors "github.com/cilium/cilium/pkg/hubble/parser/errors"
	monitorapi "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/types"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	pktSizeBytes = 100

	// testSrcIdentity and testDstIdentity are the security identities carried by the
	// cilium trace/drop notification fixtures.
	testSrcIdentity = 1234
	testDstIdentity = 5678

	// pktmonDropReasonInvalidPacket is the pktmon drop reason used by the fixtures. Its flow
	// event subtype has the second highest bit set to avoid overlap with cilium drop reasons.
	pktmonDropReasonInvalidPacket     = 2
	pktmonDropSubTypeInvalidPacket    = pktmonDropReasonInvalidPacket | (1 << 30)
	pktmonDropReasonDescInvalidPacket = flow.DropReason(pktmonDropReasonInvalidPacket)
)

var (
	errTestFailure    = errors.New("test failure")
	errModuleNotFound = errors.New("module not found")

	// testIsReplyTrue is used for flow expectations that require is_reply to be set.
	testIsReplyTrue = true
)

func makeMockEthernetIPv4TCPPacket() []byte {
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x02},
		DstMAC:       net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    net.IP{192, 168, 1, 1},
		DstIP:    net.IP{192, 168, 1, 2},
	}
	tcp := &layers.TCP{
		SrcPort: 12345,
		DstPort: 80,
		SYN:     true,
		Window:  65535,
	}

	err := tcp.SetNetworkLayerForChecksum(ip)
	if err != nil {
		panic(fmt.Sprintf("failed to set network layer for TCP: %v", err))
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	err = gopacket.SerializeLayers(buf, opts, eth, ip, tcp, gopacket.Payload([]byte{0x01, 0x02, 0x03}))
	if err != nil {
		panic(fmt.Sprintf("failed to serialize layers: %v", err))
	}

	return buf.Bytes()
}

func makeMockIPv4TCPPacket() []byte {
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    net.IP{192, 168, 1, 1},
		DstIP:    net.IP{192, 168, 1, 2},
	}
	tcp := &layers.TCP{
		SrcPort: 12345,
		DstPort: 80,
		SYN:     true,
		Window:  65535,
	}

	err := tcp.SetNetworkLayerForChecksum(ip)
	if err != nil {
		panic(fmt.Sprintf("failed to set network layer for TCP: %v", err))
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	err = gopacket.SerializeLayers(buf, opts, ip, tcp, gopacket.Payload([]byte{0x01, 0x02, 0x03}))
	if err != nil {
		panic(fmt.Sprintf("failed to serialize layers: %v", err))
	}

	return buf.Bytes()
}

func makeMockIPv6TCPPacket() []byte {
	ip := &layers.IPv6{
		Version:    6,
		HopLimit:   64,
		NextHeader: layers.IPProtocolTCP,
		SrcIP:      net.ParseIP("2001:db8::1"),
		DstIP:      net.ParseIP("2001:db8::2"),
	}
	tcp := &layers.TCP{
		SrcPort: 12345,
		DstPort: 80,
		SYN:     true,
		Window:  65535,
	}

	err := tcp.SetNetworkLayerForChecksum(ip)
	if err != nil {
		panic(fmt.Sprintf("failed to set network layer for TCP: %v", err))
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	err = gopacket.SerializeLayers(buf, opts, ip, tcp, gopacket.Payload([]byte{0x01, 0x02, 0x03}))
	if err != nil {
		panic(fmt.Sprintf("failed to serialize layers: %v", err))
	}

	return buf.Bytes()
}

func makeMockEthernetIPv4UDPPacket() []byte {
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x02},
		DstMAC:       net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    net.IP{192, 168, 1, 1},
		DstIP:    net.IP{192, 168, 1, 2},
	}
	udp := &layers.UDP{
		SrcPort: 12345,
		DstPort: 53,
	}

	err := udp.SetNetworkLayerForChecksum(ip)
	if err != nil {
		panic(fmt.Sprintf("failed to set network layer for UDP: %v", err))
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	err = gopacket.SerializeLayers(buf, opts, eth, ip, udp, gopacket.Payload([]byte{0x01, 0x02, 0x03}))
	if err != nil {
		panic(fmt.Sprintf("failed to serialize layers: %v", err))
	}

	return buf.Bytes()
}

func CheckIPv6TCPPacketFields(fl *flow.Flow, t *testing.T) {
	if fl.GetIP().GetIpVersion() != flow.IPVersion_IPv6 {
		t.Errorf("expected IP version IPv6, got %v", fl.GetIP().GetIpVersion())
	}

	if fl.GetIP().GetSource() != "2001:db8::1" {
		t.Errorf("expected source IP to be 2001:db8::1, got %v", fl.GetIP().GetSource())
	}
	if fl.GetIP().GetDestination() != "2001:db8::2" {
		t.Errorf("expected destination IP to be 2001:db8::2, got %v", fl.GetIP().GetDestination())
	}

	if fl.GetL4().GetTCP().GetSourcePort() != 12345 {
		t.Errorf("expected source port to be 12345, got %v", fl.GetL4().GetTCP().GetSourcePort())
	}
	if fl.GetL4().GetTCP().GetDestinationPort() != 80 {
		t.Errorf("expected destination port to be 80, got %v", fl.GetL4().GetTCP().GetDestinationPort())
	}
}

func CheckIPv4UDPPacketFields(fl *flow.Flow, t *testing.T) {
	if fl.GetEthernet().GetSource() != "de:ad:be:ef:00:02" {
		t.Errorf("expected source MAC to be de:ad:be:ef:00:02, got %v", fl.GetEthernet().GetSource())
	}
	if fl.GetEthernet().GetDestination() != "de:ad:be:ef:00:01" {
		t.Errorf("expected destination MAC to be de:ad:be:ef:00:01, got %v", fl.GetEthernet().GetDestination())
	}

	if fl.GetIP().GetIpVersion() != flow.IPVersion_IPv4 {
		t.Errorf("expected IP version IPv4, got %v", fl.GetIP().GetIpVersion())
	}

	if fl.GetIP().GetSource() != "192.168.1.1" {
		t.Errorf("expected source IP to be 192.168.1.1, got %v", fl.GetIP().GetSource())
	}
	if fl.GetIP().GetDestination() != "192.168.1.2" {
		t.Errorf("expected destination IP to be 192.168.1.2, got %v", fl.GetIP().GetDestination())
	}

	if fl.GetL4().GetUDP() == nil {
		t.Fatal("expected a UDP layer 4, got nil")
	}
	if fl.GetL4().GetTCP() != nil {
		t.Errorf("expected no TCP layer 4, got %v", fl.GetL4().GetTCP())
	}
	if fl.GetL4().GetUDP().GetSourcePort() != 12345 {
		t.Errorf("expected source port to be 12345, got %v", fl.GetL4().GetUDP().GetSourcePort())
	}
	if fl.GetL4().GetUDP().GetDestinationPort() != 53 {
		t.Errorf("expected destination port to be 53, got %v", fl.GetL4().GetUDP().GetDestinationPort())
	}
}

func CheckPacketFields(fl *flow.Flow, t *testing.T, checkEthFields bool) {
	if checkEthFields {
		if fl.GetEthernet().GetSource() != "de:ad:be:ef:00:02" {
			t.Errorf("expected source MAC to be de:ad:be:ef:00:02, got %v", fl.GetEthernet().GetSource())
		}

		if fl.GetEthernet().GetDestination() != "de:ad:be:ef:00:01" {
			t.Errorf("expected destination MAC to be de:ad:be:ef:00:01, got %v", fl.GetEthernet().GetDestination())
		}
	}

	if fl.GetIP().GetIpVersion() != flow.IPVersion_IPv4 {
		t.Errorf("expected IP version IPv4, got %v", fl.GetIP().GetIpVersion())
	}

	if fl.GetIP().GetSource() != "192.168.1.1" {
		t.Errorf("expected source IP to be 192.168.1.1, got %v", fl.GetIP().GetSource())
	}
	if fl.GetIP().GetDestination() != "192.168.1.2" {
		t.Errorf("expected destination IP to be 192.168.1.2, got %v", fl.GetIP().GetDestination())
	}

	if fl.GetL4().GetTCP().GetSourcePort() != 12345 {
		t.Errorf("expected source port to be 12345, got %v", fl.GetL4().GetTCP().GetSourcePort())
	}
	if fl.GetL4().GetTCP().GetDestinationPort() != 80 {
		t.Errorf("expected destination port to be 80, got %v", fl.GetL4().GetTCP().GetDestinationPort())
	}
}

// flowExpectations describes the user-visible flow semantics that advanced metrics consume.
type flowExpectations struct {
	verdict          flow.Verdict
	eventType        int32
	eventSubType     int32
	dropReasonDesc   flow.DropReason
	dropReasonExt    string // drop reason description carried in the flow extensions
	observationPoint flow.TraceObservationPoint
	trafficDirection flow.TrafficDirection
	traceReason      flow.TraceReason
	isReply          *bool // nil means the flow must not report reply information
	packetSize       uint32
}

// CheckFlowSemantics asserts the verdict, event type/subtype, drop reason, observation point,
// traffic direction, packet size extension and endpoint ownership of a decoded flow.
func CheckFlowSemantics(fl *flow.Flow, t *testing.T, exp flowExpectations) {
	t.Helper()

	if got := fl.GetVerdict(); got != exp.verdict {
		t.Errorf("expected verdict %v, got %v", exp.verdict, got)
	}
	if got := fl.GetEventType().GetType(); got != exp.eventType {
		t.Errorf("expected event type %v, got %v", exp.eventType, got)
	}
	if got := fl.GetEventType().GetSubType(); got != exp.eventSubType {
		t.Errorf("expected event subtype %v, got %v", exp.eventSubType, got)
	}
	if got := fl.GetDropReasonDesc(); got != exp.dropReasonDesc {
		t.Errorf("expected drop reason desc %v, got %v", exp.dropReasonDesc, got)
	}
	if got := utils.DropReasonDescription(fl); got != exp.dropReasonExt {
		t.Errorf("expected drop reason description %q, got %q", exp.dropReasonExt, got)
	}
	if got := fl.GetTraceObservationPoint(); got != exp.observationPoint {
		t.Errorf("expected observation point %v, got %v", exp.observationPoint, got)
	}
	if got := fl.GetTrafficDirection(); got != exp.trafficDirection {
		t.Errorf("expected traffic direction %v, got %v", exp.trafficDirection, got)
	}
	if got := fl.GetTraceReason(); got != exp.traceReason {
		t.Errorf("expected trace reason %v, got %v", exp.traceReason, got)
	}

	switch {
	case exp.isReply == nil:
		if fl.GetIsReply() != nil {
			t.Errorf("expected no reply information, got %v", fl.GetIsReply().GetValue())
		}
	case fl.GetIsReply() == nil:
		t.Errorf("expected is_reply %v, got none", *exp.isReply)
	default:
		if got := fl.GetIsReply().GetValue(); got != *exp.isReply {
			t.Errorf("expected is_reply %v, got %v", *exp.isReply, got)
		}
		if got := fl.GetReply(); got != *exp.isReply { //nolint:staticcheck // SA1019 - deprecated field kept for compatibility
			t.Errorf("expected reply %v, got %v", *exp.isReply, got)
		}
	}

	if got := utils.PacketSize(fl); got != exp.packetSize {
		t.Errorf("expected packet size %d, got %d", exp.packetSize, got)
	}

	if fl.GetSource() != nil {
		t.Errorf("expected source endpoint to remain nil before enrichment, got %v", fl.GetSource())
	}
	if fl.GetDestination() != nil {
		t.Errorf("expected destination endpoint to remain nil before enrichment, got %v", fl.GetDestination())
	}
}

func TestEmitAdvancedEvent(t *testing.T) {
	const (
		sourceIP      = "10.0.0.1"
		destinationIP = "10.0.0.2"
	)

	tests := []struct {
		name                string
		remoteContext       bool
		sourceSelected      bool
		destinationSelected bool
		wantWrite           bool
	}{
		{
			name:           "local selected source",
			sourceSelected: true,
			wantWrite:      true,
		},
		{
			name:                "local selected destination",
			destinationSelected: true,
			wantWrite:           true,
		},
		{
			name: "local neither selected",
		},
		{
			name:          "remote neither selected",
			remoteContext: true,
			wantWrite:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockFilterManager := filtermanager.NewMockIFilterManager(ctrl)
			mockEnricher := enricher.NewMockEnricherInterface(ctrl)
			event := &v1.Event{
				Event: &flow.Flow{
					IP: &flow.IP{
						Source:      sourceIP,
						Destination: destinationIP,
					},
				},
			}

			if !tt.remoteContext {
				mockFilterManager.EXPECT().HasIP(net.ParseIP(sourceIP)).Return(tt.sourceSelected)
				if !tt.sourceSelected {
					mockFilterManager.EXPECT().HasIP(net.ParseIP(destinationIP)).Return(tt.destinationSelected)
				}
			}
			if tt.wantWrite {
				mockEnricher.EXPECT().Write(event)
			}

			p := &Plugin{
				cfg: &kcfg.Config{
					RemoteContext: tt.remoteContext,
				},
				enricher:      mockEnricher,
				filterManager: mockFilterManager,
			}

			p.emitAdvancedEvent(event)
		})
	}
}

// TestHandleTraceEvent_TraceNotify invokes the handleTraceEvent function for a valid TraceNotify event
// and check if the flow object is created correctly.
func TestHandleTraceEvent_TraceNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	packet := makeMockEthernetIPv4TCPPacket()

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Fatal("expected a flow object, got nil")
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}
			CheckPacketFields(fl, t, true)
			CheckFlowSemantics(fl, t, flowExpectations{
				verdict:          flow.Verdict_FORWARDED,
				eventType:        monitorapi.MessageTypeTrace,
				eventSubType:     monitorapi.TraceToStack,
				dropReasonDesc:   flow.DropReason_DROP_REASON_UNKNOWN,
				observationPoint: flow.TraceObservationPoint_TO_STACK,
				trafficDirection: flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN,
				traceReason:      flow.TraceReason_REPLY,
				isReply:          &testIsReplyTrue,
				//nolint:gosec // ignore G115 -- packet length is within uint32 bounds in test context
				packetSize: uint32(len(packet)),
			})
			return nil
		})
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to setup logger: %v", err)
	}

	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
			RemoteContext:   true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	p.enricher = mockEnricher
	tn := TraceNotify{
		Type:     monitorapi.MessageTypeTrace,
		Version:  TraceNotifyVersion1,
		ObsPoint: monitorapi.TraceToStack,
		Reason:   TraceReasonCtReply,
		SrcLabel: testSrcIdentity,
		DstLabel: testDstIdentity,
		OrigIP:   types.IPv6{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, // example IPv6
	}

	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, tn); err != nil {
		t.Fatalf("failed to serialize TraceNotify: %v", err)
	}

	// Append mock TCP packet as payload
	buf.Write(packet)
	data := buf.Bytes()
	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil {
		t.Fatalf("expected no error for handleTraceEvent, got: %v", err)
	}
}

// TestHandleTraceEvent_DropNotify invokes the handleTraceEvent function for a valid DropNotify event
// and check if the flow object is created correctly.
func TestHandleTraceEvent_DropNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	packet := makeMockEthernetIPv4TCPPacket()

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Fatal("expected a flow object, got nil")
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}

			CheckPacketFields(fl, t, true)
			CheckFlowSemantics(fl, t, flowExpectations{
				verdict:   flow.Verdict_DROPPED,
				eventType: monitorapi.MessageTypeDrop,
				// handleTraceEvent maps the cilium drop subtype into a retina drop reason.
				eventSubType:     int32(flow.DropReason_DROP_REASON_UNKNOWN),
				dropReasonDesc:   flow.DropReason_DROP_REASON_UNKNOWN,
				dropReasonExt:    utils.DropReason_Reason_InvalidPacket.String(),
				observationPoint: flow.TraceObservationPoint_UNKNOWN_POINT,
				trafficDirection: flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN,
				traceReason:      flow.TraceReason_TRACE_REASON_UNKNOWN,
				//nolint:gosec // ignore G115 -- packet length is within uint32 bounds in test context
				packetSize: uint32(len(packet)),
			})
			return nil
		})

	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to setup logger: %v", err)
	}

	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
			RemoteContext:   true,
		},
		l: log.Logger().Named("test-ebpf"),
	}

	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	p.enricher = mockEnricher

	dn := DropNotify{
		Type:     monitorapi.MessageTypeDrop,
		Version:  DropNotifyVersion1,
		SubType:  uint8(utils.DropReason_Reason_InvalidPacket),
		SrcLabel: testSrcIdentity,
		DstLabel: testDstIdentity,
	}
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, dn); err != nil {
		t.Fatalf("failed to serialize DropNotify: %v", err)
	}

	// Append mock TCP packet as payload
	buf.Write(packet)

	data := buf.Bytes()

	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil {
		t.Fatalf("expected no error for handleTraceEvent, got: %v", err)
	}
}

// TestHandleTraceEvent_UnknownEventType_NoError tests the behavior of the handleTraceEvent function
// when an unknown event type is received.
func TestHandleTraceEvent_UnknownEventType_NoError(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to setup logger: %v", err)
	}

	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	// Create a byte array with one byte set to 4 (Unknown event type)
	data := []byte{8} // Neither TraceNotify nor DropNotify
	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil {
		t.Fatalf("expected no error for unknown event type, got: %v", err)
	}
}

// TestHandleTraceEvent_InvalidTraceNotify tests the behavior of the handleTraceEvent function
// when an invalid TraceNotify event is received.
func TestHandleTraceEvent_InvalidTraceNotify(t *testing.T) {
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	err := p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	data := []byte{monitorapi.MessageTypeTrace, 0} // Invalid TraceNotify
	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err == nil {
		t.Fatalf("expected error for invalid TraceNotify, got none")
	} else if err.Error() != "invalid size for TraceNotify: 2" {
		t.Fatalf("expected error - invalid size for TraceNotify: 2, got: %v", err)
	}
}

// TestHandleTraceEvent_InvalidDropNotify tests the behavior of the handleTraceEvent function
// when an invalid DropNotify event is received.
func TestHandleTraceEvent_InvalidDropNotify(t *testing.T) {
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	err := p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	data := []byte{monitorapi.MessageTypeDrop, 0} // Invalid DropNotify
	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err == nil {
		t.Fatalf("expected error for invalid DropNotify, got none")
	} else if err.Error() != "invalid size for DropNotify: 2" {
		t.Fatalf("expected error - invalid size for DropNotify: 2, got: %v", err)
	}
}

// TestHandleTraceEvent_InvalidPktmonDropNotify tests the behavior of the handleTraceEvent function
// when a pktmon DropNotify event shorter than the header is received.
func TestHandleTraceEvent_InvalidPktmonDropNotify(t *testing.T) {
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	err := p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	data := []byte{MessageTypePktmonDrop} // Shorter than the pktmon drop header
	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err == nil {
		t.Fatalf("expected error for invalid pktmon DropNotify, got none")
	} else if err.Error() != "invalid size for DropNotify: 1" {
		t.Fatalf("expected error - invalid size for DropNotify: 1, got: %v", err)
	}
}

// TestHandleTraceEvent_DataNil_SizeNonZero tests the behavior of the handleTraceEvent function
// when the data pointer is nil and size is non-zero.
func TestHandleTraceEvent_DataNil_SizeNonZero(t *testing.T) {
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	err := p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	var mockCiliumEventSize uint32 = 100
	err = p.handleTraceEvent(nil, mockCiliumEventSize)
	if err == nil {
		t.Fatalf("expected error - handleTraceEvent data received is nil")
	} else if err.Error() != "handleTraceEvent data received is nil" {
		t.Fatalf("expected error - handleTraceEvent data received is nil, got %v", err)
	}
}

// TestHandleTraceEvent_InvalidSizeZero tests the behavior of the handleTraceEvent function
// when the size is zero.
func TestHandleTraceEvent_InvalidSizeZero(t *testing.T) {
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	err := p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	err = p.handleTraceEvent(nil, 0)
	if err == nil {
		t.Fatalf("expected error - invalid size 0")
	} else if err.Error() != "invalid size: 0" {
		t.Fatalf("expected error - invalid size: 0, got %v", err)
	}
}

// checkGaugeValue asserts that the gauge identified by labels holds the expected value.
func checkGaugeValue(t *testing.T, gauge metrics.GaugeVec, expected float64, labels ...string) {
	t.Helper()

	g, err := gauge.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("expected a metric with labels %v, but got error %v", labels, err)
	}
	if got := testutil.ToFloat64(g); got != expected {
		t.Errorf("expected metric with labels %v to be %v, got %v", labels, expected, got)
	}
}

// TestMetricsMapIterateCallback_DropEgress tests the behavior of the metricsMapIterateCallback function
// when a drop event is received for egress traffic.
func TestMetricsMapIterateCallback_DropEgress(t *testing.T) {
	metrics.InitializeMetrics(slog.Default())
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	keyDrop := &MetricsKey{
		Version:        1,
		Reason:         DropInvalid,
		Direction:      dirEgress,
		ExtendedReason: 0, // Extended reason is not used in this test
	}
	val := &MetricsValue{Count: 3, Bytes: 3 * pktSizeBytes}
	p.metricsMapIterateCallback(keyDrop, val)
	checkGaugeValue(t, metrics.DropBytesGauge, float64(val.Bytes), "Reason_InvalidPacket", egressLabel)
	checkGaugeValue(t, metrics.DropPacketsGauge, float64(val.Count), "Reason_InvalidPacket", egressLabel)
}

// TestMetricsMapIterateCallback_DropIngress tests the behavior of the metricsMapIterateCallback function
// when a drop event is received for ingress traffic.
func TestMetricsMapIterateCallback_DropIngress(t *testing.T) {
	metrics.InitializeMetrics(slog.Default())
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	keyDrop := &MetricsKey{
		Version:        1,
		Reason:         DropInvalid,
		Direction:      dirIngress,
		ExtendedReason: 0, // Extended reason is not used in this test
	}
	val := &MetricsValue{Count: 5, Bytes: 5 * pktSizeBytes}
	p.metricsMapIterateCallback(keyDrop, val)
	checkGaugeValue(t, metrics.DropBytesGauge, float64(val.Bytes), "Reason_InvalidPacket", ingressLabel)
	checkGaugeValue(t, metrics.DropPacketsGauge, float64(val.Count), "Reason_InvalidPacket", ingressLabel)
}

// TestMetricsMapIterateCallback_DropPacketMonitorEgress tests the behavior of the
// metricsMapIterateCallback function when a pktmon drop event carrying an extended reason
// is received for egress traffic.
func TestMetricsMapIterateCallback_DropPacketMonitorEgress(t *testing.T) {
	metrics.InitializeMetrics(slog.Default())
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	keyDrop := &MetricsKey{
		Version:        1,
		Reason:         DropPacketMonitor,
		Direction:      dirEgress,
		ExtendedReason: 8, // Drop_Filtered
	}
	val := &MetricsValue{Count: 7, Bytes: 7 * pktSizeBytes}
	p.metricsMapIterateCallback(keyDrop, val)

	// The pktmon reason label carries both the base and the extended reason.
	const wantReason = "DropReason_PacketMonitor, Drop_Filtered"
	checkGaugeValue(t, metrics.DropBytesGauge, float64(val.Bytes), wantReason, egressLabel)
	checkGaugeValue(t, metrics.DropPacketsGauge, float64(val.Count), wantReason, egressLabel)
}

// TestMetricsMapIterateCallback_ForwardEgress tests the behavior of the metricsMapIterateCallback function
// when a forward event is received for egress traffic.
func TestMetricsMapIterateCallback_ForwardEgress(t *testing.T) {
	metrics.InitializeMetrics(slog.Default())
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	keyFwd := &MetricsKey{
		Version:        1,
		Reason:         0,
		Direction:      dirEgress,
		ExtendedReason: 0, // Extended reason is not used in this test
	}
	val := &MetricsValue{Count: 11, Bytes: 11 * pktSizeBytes}
	p.metricsMapIterateCallback(keyFwd, val)
	checkGaugeValue(t, metrics.ForwardBytesGauge, float64(val.Bytes), egressLabel)
	checkGaugeValue(t, metrics.ForwardPacketsGauge, float64(val.Count), egressLabel)
}

// TestMetricsMapIterateCallback_ForwardIngress tests the behavior of the metricsMapIterateCallback function
// when a forward event is received for ingress traffic.
func TestMetricsMapIterateCallback_ForwardIngress(t *testing.T) {
	metrics.InitializeMetrics(slog.Default())
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	keyFwd := &MetricsKey{
		Version:        1,
		Reason:         0,
		Direction:      dirIngress,
		ExtendedReason: 0, // Extended reason is not used in this test
	}
	val := &MetricsValue{Count: 13, Bytes: 13 * pktSizeBytes}
	p.metricsMapIterateCallback(keyFwd, val)
	checkGaugeValue(t, metrics.ForwardBytesGauge, float64(val.Bytes), ingressLabel)
	checkGaugeValue(t, metrics.ForwardPacketsGauge, float64(val.Count), ingressLabel)
}

// TestMetricsMapIterateCallback_NilKey tests the behavior of the metricsMapIterateCallback function
// when the key is nil and value is non-nil.
func TestMetricsMapIterateCallback_NilKey(t *testing.T) {
	// it should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	metrics.InitializeMetrics(slog.Default())
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	fakeValues := &MetricsValue{}
	p.metricsMapIterateCallback(nil, fakeValues)
}

// TestMetricsMapIterateCallback_NilValue tests the behavior of the metricsMapIterateCallback function
// when the value is nil.
func TestMetricsMapIterateCallback_NilValue(t *testing.T) {
	// it should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	metrics.InitializeMetrics(slog.Default())
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	key := &MetricsKey{}
	p.metricsMapIterateCallback(key, nil)
}

// TestIterateWithCallback_Error_NilMetricsValue tests the behavior of the IterateWithCallback function
// when retinaEBPFAPI invokes enumCallBack with nil value.
func TestIterateWithCallback_Error_NilMetricsValue(t *testing.T) {
	// Mock the function variable to simulate a successful Windows API call
	orig := callEnumMetricsMap
	callEnumMetricsMap = func(_ uintptr) (uintptr, uintptr, error) {
		return 0, 0, nil
	}
	defer func() { callEnumMetricsMap = orig }()

	m := NewMetricsMap()
	logger := log.Logger().Named("test-ebpf")

	called := false
	err := m.IterateWithCallback(logger, func(_ *MetricsKey, _ *MetricsValue) {
		called = true
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	fakeKey := &MetricsKey{}
	enumCallBack(unsafe.Pointer(fakeKey), nil)
	if called {
		t.Errorf("expected callback not to be called")
	}
}

// TestIterateWithCallback_Error_NilMetricsKey tests the behavior of the IterateWithCallback function
// when retinaEBPFAPI invokes enumCallBack with nil key.
func TestIterateWithCallback_Error_NilMetricsKey(t *testing.T) {
	// Mock the function variable to simulate a successful Windows API call
	orig := callEnumMetricsMap
	callEnumMetricsMap = func(_ uintptr) (uintptr, uintptr, error) {
		return 0, 0, nil
	}
	defer func() { callEnumMetricsMap = orig }()

	m := NewMetricsMap()
	logger := log.Logger().Named("test-ebpf")

	called := false
	err := m.IterateWithCallback(logger, func(_ *MetricsKey, _ *MetricsValue) {
		called = true
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	fakeValues := &MetricsValue{}
	enumCallBack(unsafe.Pointer(nil), unsafe.Pointer(fakeValues))
	if called {
		t.Errorf("expected callback not to be called")
	}
}

// TestIterateWithCallback_Error_NilKeyAndValue tests the behavior of the IterateWithCallback function
// when retinaEBPFAPI invokes enumCallBack with nil value.
func TestIterateWithCallback_Error_NilMetricValue(t *testing.T) {
	// Mock the function variable to simulate a successful Windows API call
	orig := callEnumMetricsMap
	callEnumMetricsMap = func(_ uintptr) (uintptr, uintptr, error) {
		return 0, 0, nil
	}
	defer func() { callEnumMetricsMap = orig }()

	m := NewMetricsMap()
	logger := log.Logger().Named("test-ebpf")

	called := false
	err := m.IterateWithCallback(logger, func(_ *MetricsKey, _ *MetricsValue) {
		called = true
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	fakeKey := &MetricsKey{}
	enumCallBack(unsafe.Pointer(fakeKey), unsafe.Pointer(nil))
	if called {
		t.Errorf("expected callback not to be called")
	}
}

// TestIterateWithCallback_Success tests the behavior of the IterateWithCallback function
// when retinaEBPFAPI invokes enumCallBack with valid key and value.
func TestIterateWithCallback_Success(t *testing.T) {
	// Mock the function variable to simulate a successful Windows API call
	orig := callEnumMetricsMap
	callEnumMetricsMap = func(_ uintptr) (uintptr, uintptr, error) {
		return 0, 0, nil
	}
	defer func() { callEnumMetricsMap = orig }()

	m := NewMetricsMap()
	logger := log.Logger().Named("test-ebpf")

	called := false
	err := m.IterateWithCallback(logger, func(_ *MetricsKey, _ *MetricsValue) {
		called = true
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	fakeKey := &MetricsKey{}
	fakeValues := &MetricsValue{}
	enumCallBack(unsafe.Pointer(fakeKey), unsafe.Pointer(fakeValues))
	if !called {
		t.Errorf("expected callback to be called")
	}
}

// TestUnregisterForCallback_Success tests the behavior of the UnregisterForCallback function
// when retinaEBPFAPI successfully unregisters the callback.
func TestUnregisterForCallback_Success(t *testing.T) {
	// Mock the function variable
	orig := callUnregisterEventsMapCallback
	callUnregisterEventsMapCallback = func(_ uintptr) (uintptr, uintptr, error) {
		return 0, 0, nil // Simulate success
	}
	defer func() { callUnregisterEventsMapCallback = orig }()

	em := NewEventsMap()

	err := em.UnregisterForCallback()
	if err != nil {
		t.Fatalf("expected no error when unregistering callback with eventmap, got %v", err)
	}
}

// TestUnregisterForCallback_Error tests the behavior of the UnregisterForCallback function
// when retinaEBPFAPI fails to unregister the callback.
func TestUnregisterForCallback_Error(t *testing.T) {
	// Mock the function variable to simulate an error
	orig := callUnregisterEventsMapCallback
	callUnregisterEventsMapCallback = func(_ uintptr) (uintptr, uintptr, error) {
		return 1, 0, fmt.Errorf("%w", errTestFailure)
	}
	defer func() { callUnregisterEventsMapCallback = orig }()

	em := NewEventsMap()

	err := em.UnregisterForCallback()
	if err == nil {
		t.Fatalf("expected error when unregistering callback with eventmap, got nothing")
	}
}

// TestRegisterForCallback_Success tests the behavior of the RegisterForCallback function
// when retinaEBPFAPI successfully registers the callback.
func TestRegisterForCallback_Success(t *testing.T) {
	// Mock the function variable, not the LazyProc
	orig := callRegisterEventsMapCallback
	callRegisterEventsMapCallback = func(_, _ uintptr) (uintptr, uintptr, error) {
		return 0, 0, nil // Simulate success
	}
	defer func() { callRegisterEventsMapCallback = orig }()

	logger := log.Logger().Named("test-ebpf")
	em := NewEventsMap()

	called := false
	cb := func(_ unsafe.Pointer, _ uint32) {
		called = true
	}

	err := em.RegisterForCallback(logger, cb)
	if err != nil {
		t.Fatalf("expected no error when registering callback with eventsmap, got %v", err)
	}
	// Simulate callback
	eventsCallback(nil, 0)
	if !called {
		t.Errorf("expected callback to be called from eventsmap")
	}
}

// TestRegisterForCallback_Error tests the behavior of the RegisterForCallback function
// when retinaEBPFAPI fails to register the callback.
func TestRegisterForCallback_Error(t *testing.T) {
	// Mock the function variable to simulate an error
	orig := callRegisterEventsMapCallback
	callRegisterEventsMapCallback = func(_, _ uintptr) (uintptr, uintptr, error) {
		return 1, 0, fmt.Errorf("%w", errTestFailure)
	}
	defer func() { callRegisterEventsMapCallback = orig }()

	logger := log.Logger().Named("test-ebpf")
	em := NewEventsMap()

	cb := func(_ unsafe.Pointer, _ uint32) {
		// nop
	}

	err := em.RegisterForCallback(logger, cb)
	if err == nil {
		t.Fatalf("expected error when registering callback with eventsmap, got nothing")
	}
}

func TestStart_GracefullySkipsWhenRetinaEbpfAPIMissing(t *testing.T) {
	origIsCiliumOnWindowsEnabled := isCiliumOnWindowsEnabled
	origLoadRetinaEbpfAPI := loadRetinaEbpfAPI
	isCiliumOnWindowsEnabled = func() (bool, error) {
		return true, nil
	}
	loadRetinaEbpfAPI = func() error {
		return errModuleNotFound
	}
	defer func() {
		isCiliumOnWindowsEnabled = origIsCiliumOnWindowsEnabled
		loadRetinaEbpfAPI = origLoadRetinaEbpfAPI
	}()

	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}

	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("expected plugin to skip gracefully when retinaebpfapi.dll is missing, got %v", err)
	}
}

// TestHandleTraceEventWithEthPacket_PktmonDropNotify invokes the handleTraceEvent function for a valid DropNotify event
// and check if the flow object is created correctly.
func TestHandleTraceEventWithEthPacket_PktmonDropNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	packet := makeMockEthernetIPv4TCPPacket()

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Fatal("expected a flow object, got nil")
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}

			CheckPacketFields(fl, t, true)
			CheckFlowSemantics(fl, t, flowExpectations{
				verdict:          flow.Verdict_DROPPED,
				eventType:        MessageTypePktmonDrop,
				eventSubType:     pktmonDropSubTypeInvalidPacket,
				dropReasonDesc:   pktmonDropReasonDescInvalidPacket,
				dropReasonExt:    utils.DropReason_Drop_InvalidPacket.String(),
				observationPoint: flow.TraceObservationPoint_UNKNOWN_POINT,
				trafficDirection: flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN,
				traceReason:      flow.TraceReason_TRACE_REASON_UNKNOWN,
				//nolint:gosec // ignore G115 -- packet length is within uint32 bounds in test context
				packetSize: uint32(len(packet)),
			})
			return nil
		})

	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to setup logger: %v", err)
	}

	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
			RemoteContext:   true,
		},
		l: log.Logger().Named("test-ebpf"),
	}

	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	p.enricher = mockEnricher

	pdn := [57]uint8{}
	// type 100
	pdn[0] = 0x64
	// version 1
	pdn[2] = 0x01
	pdn[3] = 0x00
	// PacketType 1
	pdn[31] = 0x01
	pdn[32] = 0x00

	// DropReason 0x000003E9
	pdn[39] = 0x02
	pdn[40] = 0x00
	pdn[41] = 0x00
	pdn[42] = 0x00
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, pdn); err != nil {
		t.Fatalf("failed to serialize DropNotify: %v", err)
	}

	// Append mock TCP packet as payload
	buf.Write(packet)

	data := buf.Bytes()

	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil {
		t.Fatalf("expected no error for handleTraceEvent, got: %v", err)
	}
}

// TestHandleTraceEventWithIpPacket_PktmonDropNotify invokes the handleTraceEvent function for a valid DropNotify event
// and check if the flow object is created correctly.
func TestHandleTraceEventWithIpPacket_PktmonDropNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	packet := makeMockIPv4TCPPacket()

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Fatal("expected a flow object, got nil")
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}

			CheckPacketFields(fl, t, false)
			CheckFlowSemantics(fl, t, flowExpectations{
				verdict:          flow.Verdict_DROPPED,
				eventType:        MessageTypePktmonDrop,
				eventSubType:     pktmonDropSubTypeInvalidPacket,
				dropReasonDesc:   pktmonDropReasonDescInvalidPacket,
				dropReasonExt:    utils.DropReason_Drop_InvalidPacket.String(),
				observationPoint: flow.TraceObservationPoint_UNKNOWN_POINT,
				trafficDirection: flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN,
				traceReason:      flow.TraceReason_TRACE_REASON_UNKNOWN,
				//nolint:gosec // ignore G115 -- packet length is within uint32 bounds in test context
				packetSize: uint32(len(packet)),
			})
			return nil
		})

	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to setup logger: %v", err)
	}

	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
			RemoteContext:   true,
		},
		l: log.Logger().Named("test-ebpf"),
	}

	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	p.enricher = mockEnricher

	// Pktmon events use packed structs for the packet headers, manually constructing test packet
	pdn := [57]uint8{}
	// type 100
	pdn[0] = 0x64
	// version 1
	pdn[2] = 0x01
	pdn[3] = 0x00
	// PacketType 3
	pdn[31] = 0x03
	pdn[32] = 0x00

	// DropReason 0x00000002
	pdn[39] = 0x02
	pdn[40] = 0x00
	pdn[41] = 0x00
	pdn[42] = 0x00
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, pdn); err != nil {
		t.Fatalf("failed to serialize DropNotify: %v", err)
	}

	// Append mock TCP packet as payload
	buf.Write(packet)

	data := buf.Bytes()

	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil {
		t.Fatalf("expected no error for handleTraceEvent, got: %v", err)
	}
}

// TestHandleTraceEventWithIPv6Packet_PktmonDropNotify invokes the handleTraceEvent function for a valid
// pktmon DropNotify event carrying a raw IPv6 payload and checks that the IPv6 branch of the parser
// populates the flow correctly.
func TestHandleTraceEventWithIPv6Packet_PktmonDropNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	packet := makeMockIPv6TCPPacket()

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Fatal("expected a flow object, got nil")
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}

			CheckIPv6TCPPacketFields(fl, t)
			CheckFlowSemantics(fl, t, flowExpectations{
				verdict:          flow.Verdict_DROPPED,
				eventType:        MessageTypePktmonDrop,
				eventSubType:     pktmonDropSubTypeInvalidPacket,
				dropReasonDesc:   pktmonDropReasonDescInvalidPacket,
				dropReasonExt:    utils.DropReason_Drop_InvalidPacket.String(),
				observationPoint: flow.TraceObservationPoint_UNKNOWN_POINT,
				trafficDirection: flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN,
				traceReason:      flow.TraceReason_TRACE_REASON_UNKNOWN,
				//nolint:gosec // ignore G115 -- packet length is within uint32 bounds in test context
				packetSize: uint32(len(packet)),
			})
			return nil
		})

	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to setup logger: %v", err)
	}

	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
			RemoteContext:   true,
		},
		l: log.Logger().Named("test-ebpf"),
	}

	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	p.enricher = mockEnricher

	// Pktmon events use packed structs for the packet headers, manually constructing test packet
	pdn := [57]uint8{}
	// type 100
	pdn[0] = 0x64
	// version 1
	pdn[2] = 0x01
	pdn[3] = 0x00
	// PacketType 3 (raw IP)
	pdn[31] = 0x03
	pdn[32] = 0x00

	// DropReason 0x00000002
	pdn[39] = 0x02
	pdn[40] = 0x00
	pdn[41] = 0x00
	pdn[42] = 0x00
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, pdn); err != nil {
		t.Fatalf("failed to serialize DropNotify: %v", err)
	}

	// Append mock IPv6/TCP packet as payload
	buf.Write(packet)

	data := buf.Bytes()

	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil {
		t.Fatalf("expected no error for handleTraceEvent, got: %v", err)
	}
}

// TestHandleTraceEventWithUDPPacket_PktmonDropNotify invokes the handleTraceEvent function for a valid
// pktmon DropNotify event carrying an Ethernet/IPv4/UDP payload and checks that the UDP layer 4 is
// decoded into the flow.
func TestHandleTraceEventWithUDPPacket_PktmonDropNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	packet := makeMockEthernetIPv4UDPPacket()

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Fatal("expected a flow object, got nil")
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}

			CheckIPv4UDPPacketFields(fl, t)
			CheckFlowSemantics(fl, t, flowExpectations{
				verdict:          flow.Verdict_DROPPED,
				eventType:        MessageTypePktmonDrop,
				eventSubType:     pktmonDropSubTypeInvalidPacket,
				dropReasonDesc:   pktmonDropReasonDescInvalidPacket,
				dropReasonExt:    utils.DropReason_Drop_InvalidPacket.String(),
				observationPoint: flow.TraceObservationPoint_UNKNOWN_POINT,
				trafficDirection: flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN,
				traceReason:      flow.TraceReason_TRACE_REASON_UNKNOWN,
				//nolint:gosec // ignore G115 -- packet length is within uint32 bounds in test context
				packetSize: uint32(len(packet)),
			})
			return nil
		})

	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to setup logger: %v", err)
	}

	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
			RemoteContext:   true,
		},
		l: log.Logger().Named("test-ebpf"),
	}

	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	p.enricher = mockEnricher

	// Pktmon events use packed structs for the packet headers, manually constructing test packet
	pdn := [57]uint8{}
	// type 100
	pdn[0] = 0x64
	// version 1
	pdn[2] = 0x01
	pdn[3] = 0x00
	// PacketType 1 (Ethernet)
	pdn[31] = 0x01
	pdn[32] = 0x00

	// DropReason 0x00000002
	pdn[39] = 0x02
	pdn[40] = 0x00
	pdn[41] = 0x00
	pdn[42] = 0x00
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, pdn); err != nil {
		t.Fatalf("failed to serialize DropNotify: %v", err)
	}

	// Append mock UDP packet as payload
	buf.Write(packet)

	data := buf.Bytes()

	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil {
		t.Fatalf("expected no error for handleTraceEvent, got: %v", err)
	}
}

// makePktmonDropHeader builds a pktmon drop notification header of the exact supported length.
func makePktmonDropHeader(version, packetType uint16) []byte {
	header := make([]byte, dropPktmonNotifyV1Len)
	header[0] = MessageTypePktmonDrop
	byteorder.Native.PutUint16(header[2:4], version)
	byteorder.Native.PutUint16(header[31:33], packetType)
	byteorder.Native.PutUint32(header[39:43], 2)
	return header
}

// makeDropNotifyHeader builds a cilium drop notification header of the exact supported length.
func makeDropNotifyHeader(version uint16) []byte {
	header := make([]byte, dropNotifyV1Len)
	header[0] = monitorapi.MessageTypeDrop
	byteorder.Native.PutUint16(header[14:16], version)
	return header
}

// makeTraceNotifyHeader builds a trace notification header of the given length.
func makeTraceNotifyHeader(version uint16, length int) []byte {
	header := make([]byte, length)
	header[0] = monitorapi.MessageTypeTrace
	byteorder.Native.PutUint16(header[14:16], version)
	return header
}

// TestDecodeNotify_MalformedHeaders feeds truncated, exact-length and unsupported-version
// buffers straight into the packed-struct decoders and asserts they return an error
// (or succeed for the exact supported length) without panicking.
func TestDecodeNotify_MalformedHeaders(t *testing.T) {
	decodePktmon := func(data []byte) error { return DecodePktmonDrop(data, &PktmonDropNotify{}) }
	decodeDrop := func(data []byte) error { return DecodeDropNotify(data, &DropNotify{}) }
	decodeTrace := func(data []byte) error { return DecodeTraceNotify(data, &TraceNotify{}) }

	tests := []struct {
		name    string
		decode  func([]byte) error
		data    []byte
		wantErr error
	}{
		{
			name:    "pktmon drop of length 1",
			decode:  decodePktmon,
			data:    []byte{MessageTypePktmonDrop},
			wantErr: errUnexpectedDropNotifyLength,
		},
		{
			name:    "pktmon drop one byte short",
			decode:  decodePktmon,
			data:    makePktmonDropHeader(DropNotifyVersion1, uint16(PktMonPayloadEthernet))[:dropPktmonNotifyV1Len-1],
			wantErr: errUnexpectedDropNotifyLength,
		},
		{
			name:   "pktmon drop of exact supported length",
			decode: decodePktmon,
			data:   makePktmonDropHeader(DropNotifyVersion1, uint16(PktMonPayloadEthernet)),
		},
		{
			name:    "pktmon drop with unsupported version",
			decode:  decodePktmon,
			data:    makePktmonDropHeader(DropNotifyVersion2, uint16(PktMonPayloadEthernet)),
			wantErr: errInvalidPktmonDropNotifyVersion,
		},
		{
			name:    "drop notify of length 1",
			decode:  decodeDrop,
			data:    []byte{monitorapi.MessageTypeDrop},
			wantErr: errUnexpectedDropNotifyLength,
		},
		{
			name:    "drop notify one byte short",
			decode:  decodeDrop,
			data:    makeDropNotifyHeader(DropNotifyVersion1)[:dropNotifyV1Len-1],
			wantErr: errUnexpectedDropNotifyLength,
		},
		{
			name:   "drop notify of exact supported length",
			decode: decodeDrop,
			data:   makeDropNotifyHeader(DropNotifyVersion1),
		},
		{
			name:    "drop notify with unsupported version",
			decode:  decodeDrop,
			data:    makeDropNotifyHeader(DropNotifyVersion2),
			wantErr: errInvalidDropNotifyVersion,
		},
		{
			name:    "trace notify of length 1",
			decode:  decodeTrace,
			data:    []byte{monitorapi.MessageTypeTrace},
			wantErr: errTraceNotifyLength,
		},
		{
			name:   "trace notify v0 of exact supported length",
			decode: decodeTrace,
			data:   makeTraceNotifyHeader(TraceNotifyVersion0, traceNotifyV0Len),
		},
		{
			name:    "trace notify v1 truncated to the v0 length",
			decode:  decodeTrace,
			data:    makeTraceNotifyHeader(TraceNotifyVersion1, traceNotifyV0Len),
			wantErr: errTraceNotifyLength,
		},
		{
			name:   "trace notify v1 of exact supported length",
			decode: decodeTrace,
			data:   makeTraceNotifyHeader(TraceNotifyVersion1, traceNotifyV1Len),
		},
		{
			name:    "trace notify with unsupported version",
			decode:  decodeTrace,
			data:    makeTraceNotifyHeader(TraceNotifyVersion1+1, traceNotifyV1Len),
			wantErr: errUnrecognizedTraceEvent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("unexpected panic decoding %d bytes: %v", len(tt.data), r)
				}
			}()

			err := tt.decode(tt.data)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

// TestParserDecode_MalformedPayloads feeds malformed payloads to the parser and asserts that
// decoding fails with the expected error instead of panicking on the packed/unsafe data.
func TestParserDecode_MalformedPayloads(t *testing.T) {
	ethPacket := makeMockEthernetIPv4TCPPacket()

	tests := []struct {
		name        string
		data        []byte
		wantErr     error
		wantErrText string
		wantNoError bool
	}{
		{
			name:    "empty payload",
			data:    []byte{},
			wantErr: hubbleerrors.ErrEmptyData,
		},
		{
			name:        "pktmon drop of length 1",
			data:        []byte{MessageTypePktmonDrop},
			wantErr:     errUnexpectedDropNotifyLength,
			wantErrText: "failed to parse pktmon drop",
		},
		{
			name:        "unknown event type",
			data:        []byte{8},
			wantErrText: "invalid event type",
		},
		{
			name:        "pktmon drop header with no packet payload",
			data:        makePktmonDropHeader(DropNotifyVersion1, uint16(PktMonPayloadEthernet)),
			wantNoError: true,
		},
		{
			name:        "pktmon ethernet payload truncated mid header",
			data:        append(makePktmonDropHeader(DropNotifyVersion1, uint16(PktMonPayloadEthernet)), ethPacket[:6]...),
			wantErrText: "decode layers failed",
		},
		{
			name:    "pktmon ip payload with invalid ip version nibble",
			data:    append(makePktmonDropHeader(DropNotifyVersion1, uint16(PktMonPayloadIP)), 0x50, 0x00, 0x00, 0x00),
			wantErr: errUnsupportedIPPacketType,
		},
		{
			name:    "pktmon payload with unsupported packet type",
			data:    append(makePktmonDropHeader(DropNotifyVersion1, uint16(PktMonPayloadWiFi)), ethPacket...),
			wantErr: errUnsupportedPktmonPacketType,
		},
		{
			name:        "drop notify with truncated ethernet payload",
			data:        append(makeDropNotifyHeader(DropNotifyVersion1), ethPacket[:6]...),
			wantErrText: "decode layers failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("unexpected panic decoding %d bytes: %v", len(tt.data), r)
				}
			}()

			parser, err := NewParser(slog.Default())
			if err != nil {
				t.Fatalf("failed to create parser: %v", err)
			}

			event, err := parser.Decode(&observerTypes.MonitorEvent{
				Payload: &observerTypes.PerfEvent{Data: tt.data},
			})

			if tt.wantNoError {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if event.GetFlow() == nil {
					t.Fatal("expected a flow object, got nil")
				}
				if event.GetFlow().GetIP() != nil {
					t.Errorf("expected no IP layer for a header-only event, got %v", event.GetFlow().GetIP())
				}
				return
			}

			if err == nil {
				t.Fatalf("expected an error, got none")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got: %v", tt.wantErr, err)
			}
			if tt.wantErrText != "" && !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErrText, err)
			}
		})
	}
}

var errRegistryFailure = errors.New("registry failure")

// TestStart_CiliumRegistryMatrix asserts ebpfwindows only starts when Cilium on Windows is
// enabled. It is the complement of the equivalent hnsstats matrix: for every registry state
// exactly one of the two plugins may start.
func TestStart_CiliumRegistryMatrix(t *testing.T) {
	tests := []struct {
		name          string
		ciliumEnabled bool
		ciliumErr     error
		wantStarted   bool
		wantErr       error
	}{
		{
			name: "registry value missing or 0 skips ebpfwindows in favour of hnsstats",
		},
		{
			name:          "registry value 1 starts ebpfwindows",
			ciliumEnabled: true,
			wantStarted:   true,
		},
		{
			name:      "registry read error fails ebpfwindows",
			ciliumErr: errRegistryFailure,
			wantErr:   errRegistryFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origIsCiliumOnWindowsEnabled := isCiliumOnWindowsEnabled
			origLoadRetinaEbpfAPI := loadRetinaEbpfAPI
			origPullMetricsAndEvents := pullMetricsAndEventsFn
			t.Cleanup(func() {
				isCiliumOnWindowsEnabled = origIsCiliumOnWindowsEnabled
				loadRetinaEbpfAPI = origLoadRetinaEbpfAPI
				pullMetricsAndEventsFn = origPullMetricsAndEvents
			})

			isCiliumOnWindowsEnabled = func() (bool, error) { return tt.ciliumEnabled, tt.ciliumErr }
			loadRetinaEbpfAPI = func() error { return nil }

			started := false
			pullMetricsAndEventsFn = func(*Plugin, context.Context) {
				started = true
			}

			p := &Plugin{
				cfg: &kcfg.Config{
					MetricsInterval: 100 * time.Second,
					EnablePodLevel:  true,
				},
				l: log.Logger().Named("test-ebpf"),
			}

			err := p.Start(context.Background())

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantStarted, started, "ebpfwindows metrics collection start decision")
		})
	}
}
