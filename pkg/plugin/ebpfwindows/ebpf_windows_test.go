// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package ebpfwindows

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
	"unsafe"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	monitorapi "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/types"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"go.uber.org/mock/gomock"
)

const (
	pktSizeBytes = 100
)

var errTestFailure = errors.New("test failure")

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

// TestHandleTraceEvent_TraceNotify invokes the handleTraceEvent function for a valid TraceNotify event
// and check if the flow object is created correctly.
func TestHandleTraceEvent_TraceNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Error("expected a flow object, got nil")
			}
			eventType := fl.GetEventType().GetType()
			if eventType != monitorapi.MessageTypeTrace {
				t.Errorf("expected event type %v, got %v", monitorapi.MessageTypeTrace, eventType)
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}
			CheckPacketFields(fl, t, true)
			// Add more assertions as needed
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
		},
		l: log.Logger().Named("test-ebpf"),
	}
	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	p.enricher = mockEnricher
	tn := TraceNotify{
		Type:    monitorapi.MessageTypeTrace,
		Version: TraceNotifyVersion1,
		OrigIP:  types.IPv6{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, // example IPv6
	}

	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, tn); err != nil {
		t.Fatalf("failed to serialize TraceNotify: %v", err)
	}

	// Append mock TCP packet as payload
	packet := makeMockEthernetIPv4TCPPacket()
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

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Error("expected a flow object, got nil")
			}
			subType := fl.GetEventType().GetType()
			if subType != monitorapi.MessageTypeDrop {
				t.Errorf("expected event type %v, got %v", monitorapi.MessageTypeDrop, subType)
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}

			CheckPacketFields(fl, t, true)
			// Add more assertions as needed
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
		},
		l: log.Logger().Named("test-ebpf"),
	}

	err = p.Init()
	if err != nil {
		t.Fatalf("failed to initialize plugin: %v", err)
	}

	p.enricher = mockEnricher

	dn := DropNotify{
		Type:    monitorapi.MessageTypeDrop,
		Version: DropNotifyVersion1,
	}
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, dn); err != nil {
		t.Fatalf("failed to serialize DropNotify: %v", err)
	}

	// Append mock TCP packet as payload
	packet := makeMockEthernetIPv4TCPPacket()
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

// TestMetricsMapIterateCallback_DropEgress tests the behavior of the metricsMapIterateCallback function
// when a drop event is received for egress traffic.
func TestMetricsMapIterateCallback_DropEgress(t *testing.T) {
	metrics.InitializeMetrics()
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	keyDrop := &MetricsKey{
		Version:        1,
		Reason:         2,
		Direction:      dirEgress,
		ExtendedReason: 0, // Extended reason is not used in this test
	}
	val := &MetricsValue{Count: 1, Bytes: pktSizeBytes}
	p.metricsMapIterateCallback(keyDrop, val)
	_, err := metrics.DropBytesGauge.GetMetricWithLabelValues("Reason_InvalidPacket", "egress")
	if err != nil {
		t.Fatalf("expected a dropbyteguage metric with label Reason_InvalidPacket and egress, but got error %v", err)
	}
	_, err = metrics.DropPacketsGauge.GetMetricWithLabelValues("Reason_InvalidPacket", "egress")
	if err != nil {
		t.Fatalf("expected a droppktguage metrics with label Reason_InvalidPacket and egress, but got error %v", err)
	}
}

// TestMetricsMapIterateCallback_DropIngress tests the behavior of the metricsMapIterateCallback function
// when a drop event is received for ingress traffic.
func TestMetricsMapIterateCallback_DropIngress(t *testing.T) {
	metrics.InitializeMetrics()
	p := &Plugin{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named("test-ebpf"),
	}
	keyDrop := &MetricsKey{
		Version:        1,
		Reason:         2,
		Direction:      dirIngress,
		ExtendedReason: 0, // Extended reason is not used in this test
	}
	val := &MetricsValue{Count: 1, Bytes: pktSizeBytes}
	p.metricsMapIterateCallback(keyDrop, val)
	_, err := metrics.DropBytesGauge.GetMetricWithLabelValues("Reason_InvalidPacket", "ingress")
	if err != nil {
		t.Fatalf("expected a dropbyteguage metric with label Reason_InvalidPacket and ingress, but got error %v", err)
	}
	_, err = metrics.DropPacketsGauge.GetMetricWithLabelValues("Reason_InvalidPacket", "ingress")
	if err != nil {
		t.Fatalf("expected a droppktguage metrics with label Reason_InvalidPacket and ingress, but got error %v", err)
	}
}

// TestMetricsMapIterateCallback_ForwardEgress tests the behavior of the metricsMapIterateCallback function
// when a forward event is received for egress traffic.
func TestMetricsMapIterateCallback_ForwardEgress(t *testing.T) {
	metrics.InitializeMetrics()
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
	val := &MetricsValue{Count: 1, Bytes: pktSizeBytes}
	p.metricsMapIterateCallback(keyFwd, val)
	_, err := metrics.ForwardBytesGauge.GetMetricWithLabelValues("egress")
	if err != nil {
		t.Fatalf("expected a fwdbyteguage metric with label egress, but got error %v", err)
	}
	_, err = metrics.ForwardPacketsGauge.GetMetricWithLabelValues("egress")
	if err != nil {
		t.Fatalf("expected a fwdpktguage metrics with label egress, but got error %v", err)
	}
}

// TestMetricsMapIterateCallback_ForwardIngress tests the behavior of the metricsMapIterateCallback function
// when a forward event is received for ingress traffic.
func TestMetricsMapIterateCallback_ForwardIngress(t *testing.T) {
	metrics.InitializeMetrics()
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
	val := &MetricsValue{Count: 1, Bytes: pktSizeBytes}
	p.metricsMapIterateCallback(keyFwd, val)
	_, err := metrics.ForwardBytesGauge.GetMetricWithLabelValues("ingress")
	if err != nil {
		t.Fatalf("expected a fwdbyteguage with label ingress, but got error %v", err)
	}
	_, err = metrics.ForwardPacketsGauge.GetMetricWithLabelValues("ingress")
	if err != nil {
		t.Fatalf("expected a fwdpktguage with label ingress, but got error %v", err)
	}
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

	metrics.InitializeMetrics()
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

	metrics.InitializeMetrics()
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

// TestHandleTraceEventWithEthPacket_PktmonDropNotify invokes the handleTraceEvent function for a valid DropNotify event
// and check if the flow object is created correctly.
func TestHandleTraceEventWithEthPacket_PktmonDropNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Error("expected a flow object, got nil")
			}
			eventType := fl.GetEventType().GetType()
			if eventType != MessageTypePktmonDrop {
				t.Errorf("expected event type %v, got %v", MessageTypePktmonDrop, eventType)
			}

			var testDropReason int32 = 1001
			eventSubType := fl.GetEventType().GetSubType()
			if eventSubType != testDropReason {
				t.Errorf("expected event type %v, got %v", testDropReason, eventSubType)
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}

			CheckPacketFields(fl, t, true)
			// Add more assertions as needed
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
	pdn[39] = 0xE9
	pdn[40] = 0x03
	pdn[41] = 0x00
	pdn[42] = 0x00
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.LittleEndian, pdn); err != nil {
		t.Fatalf("failed to serialize DropNotify: %v", err)
	}

	// Append mock TCP packet as payload
	packet := makeMockEthernetIPv4TCPPacket()
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

	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(event *v1.Event) error {
			fl := event.GetFlow()
			if fl == nil {
				t.Error("expected a flow object, got nil")
			}
			eventType := fl.GetEventType().GetType()
			if eventType != MessageTypePktmonDrop {
				t.Errorf("expected event type %v, got %v", MessageTypePktmonDrop, eventType)
			}

			var testDropReason int32 = 2
			eventSubType := fl.GetEventType().GetSubType()
			if eventSubType != testDropReason {
				t.Errorf("expected event type %v, got %v", testDropReason, eventSubType)
			}

			if fl.GetType() != flow.FlowType_L3_L4 {
				t.Errorf("expected flow type L3_L4, got %v", fl.GetType())
			}

			CheckPacketFields(fl, t, false)
			// Add more assertions as needed
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
	packet := makeMockIPv4TCPPacket()
	buf.Write(packet)

	data := buf.Bytes()

	//nolint:gosec // ignore G115 -- data length is guaranteed to be within uint32 bounds in test context
	err = p.handleTraceEvent(unsafe.Pointer(&data[0]), uint32(len(data)))
	if err != nil {
		t.Fatalf("expected no error for handleTraceEvent, got: %v", err)
	}
}
