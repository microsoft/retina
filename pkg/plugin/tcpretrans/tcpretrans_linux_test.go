// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package tcpretrans

import (
	"log/slog"
	"net"
	"os"
	"testing"
	"unsafe"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf/perf"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMain(m *testing.M) {
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics(slog.Default())
	os.Exit(m.Run())
}

// --- helpers ---

// buildTestRecord serialises a tcpretransTcpretransEvent into a perf.Record
// using the same memory layout the BPF perf ring produces.
func buildTestRecord(event tcpretransTcpretransEvent) perf.Record {
	eventSize := int(unsafe.Sizeof(event))
	eventBytes := unsafe.Slice((*byte)(unsafe.Pointer(&event)), eventSize) //nolint:gosec // test-only
	buf := make([]byte, eventSize)
	copy(buf, eventBytes)
	return perf.Record{RawSample: buf}
}

// ipv4Native converts a dotted-quad string to the uint32 representation
// stored by the BPF program (network byte order bytes read as LE uint32).
func ipv4Native(s string) uint32 {
	ip := net.ParseIP(s).To4()
	return *(*uint32)(unsafe.Pointer(&ip[0])) //nolint:gosec // test-only
}

// ipv6Bytes converts an IPv6 string to the [16]uint8 stored by the BPF program.
func ipv6Bytes(s string) [16]uint8 {
	var b [16]uint8
	copy(b[:], net.ParseIP(s).To16())
	return b
}

// newTestPlugin returns a minimal tcpretrans for unit testing with an
// external channel to receive emitted events.
func newTestPlugin(ch chan *v1.Event) *tcpretrans {
	return &tcpretrans{
		cfg:             &kcfg.Config{EnablePodLevel: true},
		l:               log.Logger().Named(name),
		externalChannel: ch,
	}
}

// --- constructor / lifecycle ---

func TestNew(t *testing.T) {
	cfg := &kcfg.Config{EnablePodLevel: true}
	p := New(cfg)
	require.NotNil(t, p)
	assert.Equal(t, name, p.Name())
}

func TestSetupChannel(t *testing.T) {
	p := &tcpretrans{
		cfg: &kcfg.Config{EnablePodLevel: true},
		l:   log.Logger().Named(name),
	}
	ch := make(chan *v1.Event, 10)
	require.NoError(t, p.SetupChannel(ch))
	assert.Equal(t, ch, p.externalChannel)
}

func TestStop_PodLevelDisabled(t *testing.T) {
	p := &tcpretrans{
		cfg: &kcfg.Config{EnablePodLevel: false},
		l:   log.Logger().Named(name),
	}
	require.NoError(t, p.Stop())
}

func TestStop_NilResources(t *testing.T) {
	// Stop() must not panic when Init() was never called (all resources nil).
	p := &tcpretrans{
		cfg: &kcfg.Config{EnablePodLevel: true},
		l:   log.Logger().Named(name),
	}
	require.NoError(t, p.Stop())
}

// --- flagBit ---

func TestFlagBit(t *testing.T) {
	tests := []struct {
		name     string
		flags    uint8
		bit      uint8
		expected uint16
	}{
		{"SYN set", 0x02, 0x02, 1},
		{"SYN clear", 0x00, 0x02, 0},
		{"ACK set", 0x10, 0x10, 1},
		{"ACK clear", 0x02, 0x10, 0},
		{"FIN set", 0x01, 0x01, 1},
		{"RST set", 0x04, 0x04, 1},
		{"PSH set", 0x08, 0x08, 1},
		{"URG set", 0x20, 0x20, 1},
		{"ECE set", 0x40, 0x40, 1},
		{"CWR set", 0x80, 0x80, 1},
		{"all set", 0xFF, 0x02, 1},
		{"all set check FIN", 0xFF, 0x01, 1},
		{"none set", 0x00, 0xFF, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, flagBit(tt.flags, tt.bit))
		})
	}
}

// --- handleTCPRetransEvent ---

func TestHandleTCPRetransEvent_IPv4(t *testing.T) {
	ch := make(chan *v1.Event, 10)
	p := newTestPlugin(ch)

	record := buildTestRecord(tcpretransTcpretransEvent{
		Timestamp: 1000,
		SrcIp:     ipv4Native("10.0.0.1"),
		DstIp:     ipv4Native("10.0.0.2"),
		SrcPort:   12345,
		DstPort:   80,
		Tcpflags:  0x12, // SYN+ACK
		Af:        4,
	})

	p.handleTCPRetransEvent(record)

	require.Len(t, ch, 1)
	ev := <-ch
	fl := ev.GetFlow()
	require.NotNil(t, fl)

	assert.Equal(t, "10.0.0.1", fl.GetIP().GetSource())
	assert.Equal(t, "10.0.0.2", fl.GetIP().GetDestination())
	assert.Equal(t, uint32(12345), fl.GetL4().GetTCP().GetSourcePort())
	assert.Equal(t, uint32(80), fl.GetL4().GetTCP().GetDestinationPort())

	flags := fl.GetL4().GetTCP().GetFlags()
	require.NotNil(t, flags)
	assert.True(t, flags.GetSYN(), "SYN should be set")
	assert.True(t, flags.GetACK(), "ACK should be set")
	assert.False(t, flags.GetFIN(), "FIN should not be set")
	assert.False(t, flags.GetRST(), "RST should not be set")
}

func TestHandleTCPRetransEvent_IPv6(t *testing.T) {
	ch := make(chan *v1.Event, 10)
	p := newTestPlugin(ch)

	srcIP := "fd00::1"
	dstIP := "fd00::2"

	record := buildTestRecord(tcpretransTcpretransEvent{
		Timestamp: 2000,
		SrcIp6:    ipv6Bytes(srcIP),
		DstIp6:    ipv6Bytes(dstIP),
		SrcPort:   44444,
		DstPort:   443,
		Tcpflags:  0x10, // ACK
		Af:        6,
	})

	p.handleTCPRetransEvent(record)

	require.Len(t, ch, 1)
	ev := <-ch
	fl := ev.GetFlow()
	require.NotNil(t, fl)

	assert.Equal(t, srcIP, fl.GetIP().GetSource())
	assert.Equal(t, dstIP, fl.GetIP().GetDestination())
	assert.Equal(t, uint32(44444), fl.GetL4().GetTCP().GetSourcePort())
	assert.Equal(t, uint32(443), fl.GetL4().GetTCP().GetDestinationPort())
}

func TestHandleTCPRetransEvent_AllFlags(t *testing.T) {
	ch := make(chan *v1.Event, 10)
	p := newTestPlugin(ch)

	record := buildTestRecord(tcpretransTcpretransEvent{
		Timestamp: 3000,
		SrcIp:     ipv4Native("1.2.3.4"),
		DstIp:     ipv4Native("5.6.7.8"),
		SrcPort:   1000,
		DstPort:   2000,
		Tcpflags:  0xFF,
		Af:        4,
	})

	p.handleTCPRetransEvent(record)

	require.Len(t, ch, 1)
	flags := (<-ch).GetFlow().GetL4().GetTCP().GetFlags()
	require.NotNil(t, flags)

	assert.True(t, flags.GetFIN(), "FIN")
	assert.True(t, flags.GetSYN(), "SYN")
	assert.True(t, flags.GetRST(), "RST")
	assert.True(t, flags.GetPSH(), "PSH")
	assert.True(t, flags.GetACK(), "ACK")
	assert.True(t, flags.GetURG(), "URG")
	assert.True(t, flags.GetECE(), "ECE")
	assert.True(t, flags.GetCWR(), "CWR")
	assert.False(t, flags.GetNS(), "NS is never set from tcp_skb_cb")
}

func TestHandleTCPRetransEvent_NoFlags(t *testing.T) {
	ch := make(chan *v1.Event, 10)
	p := newTestPlugin(ch)

	record := buildTestRecord(tcpretransTcpretransEvent{
		Timestamp: 4000,
		SrcIp:     ipv4Native("1.2.3.4"),
		DstIp:     ipv4Native("5.6.7.8"),
		SrcPort:   1000,
		DstPort:   2000,
		Tcpflags:  0x00,
		Af:        4,
	})

	p.handleTCPRetransEvent(record)

	require.Len(t, ch, 1)
	flags := (<-ch).GetFlow().GetL4().GetTCP().GetFlags()
	require.NotNil(t, flags)

	assert.False(t, flags.GetFIN())
	assert.False(t, flags.GetSYN())
	assert.False(t, flags.GetRST())
	assert.False(t, flags.GetPSH())
	assert.False(t, flags.GetACK())
	assert.False(t, flags.GetURG())
	assert.False(t, flags.GetECE())
	assert.False(t, flags.GetCWR())
}

func TestHandleTCPRetransEvent_TruncatedRecord(t *testing.T) {
	ch := make(chan *v1.Event, 10)
	p := newTestPlugin(ch)

	// Record shorter than the event struct — must be silently dropped.
	record := perf.Record{RawSample: []byte{0x01, 0x02, 0x03}}
	p.handleTCPRetransEvent(record)

	assert.Empty(t, ch, "truncated record should not emit an event")
}

func TestHandleTCPRetransEvent_UnknownAF(t *testing.T) {
	ch := make(chan *v1.Event, 10)
	p := newTestPlugin(ch)

	record := buildTestRecord(tcpretransTcpretransEvent{
		Timestamp: 5000,
		Af:        99, // unsupported address family
	})

	p.handleTCPRetransEvent(record)

	assert.Empty(t, ch, "unknown AF should not emit an event")
}

func TestHandleTCPRetransEvent_WithEnricher(t *testing.T) {
	ctrl := gomock.NewController(t)

	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck // generated mock
	menricher.EXPECT().Write(gomock.Any()).Times(1)

	ch := make(chan *v1.Event, 10)
	p := newTestPlugin(ch)
	p.enricher = menricher

	record := buildTestRecord(tcpretransTcpretransEvent{
		Timestamp: 6000,
		SrcIp:     ipv4Native("10.0.0.1"),
		DstIp:     ipv4Native("10.0.0.2"),
		SrcPort:   5555,
		DstPort:   80,
		Tcpflags:  0x10,
		Af:        4,
	})

	p.handleTCPRetransEvent(record)

	require.Len(t, ch, 1)
}

func TestHandleTCPRetransEvent_ChannelFull(t *testing.T) {
	// When the external channel is full, the event is dropped (not blocking).
	ch := make(chan *v1.Event) // unbuffered — will always be full
	p := newTestPlugin(ch)

	record := buildTestRecord(tcpretransTcpretransEvent{
		Timestamp: 7000,
		SrcIp:     ipv4Native("10.0.0.1"),
		DstIp:     ipv4Native("10.0.0.2"),
		SrcPort:   1111,
		DstPort:   2222,
		Tcpflags:  0x02,
		Af:        4,
	})

	// Must not block.
	p.handleTCPRetransEvent(record)

	// Channel still empty (nobody reading), event was dropped.
	assert.Empty(t, ch)
}
