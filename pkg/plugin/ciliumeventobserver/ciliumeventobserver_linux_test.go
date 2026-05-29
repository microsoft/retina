package ciliumeventobserver

//nolint:typecheck // do not need for test
import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/testutils"
	"github.com/cilium/cilium/pkg/monitor"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/monitor/payload"
	"github.com/gopacket/gopacket/layers"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/pubsub"
	"gotest.tools/assert"
)

func TestStartError(t *testing.T) {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())

	c := cache.New(pubsub.New())
	e := enricher.New(ctxTimeout, c)
	e.Run()
	defer e.Reader.Close()

	cfg := &config.Config{
		EnablePodLevel: true,
	}
	cil := New(cfg)
	_ = cil.Init()
	md := NewMockDialer(true)
	cil.(*ciliumeventobserver).d = md

	err := cil.Start(ctxTimeout)
	assert.Assert(t, err != nil, "Error should not be nil")
}

func TestStart(t *testing.T) {
	ctxWithCancel, cancel := context.WithCancel(context.Background())
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())

	cfg := &config.Config{
		EnablePodLevel: true,
	}
	cil := New(cfg)
	exChan := make(chan *v1.Event, 1)
	_ = cil.SetupChannel(exChan)
	_ = cil.Init()
	md := NewMockDialer(false)
	cil.(*ciliumeventobserver).d = md
	cil.(*ciliumeventobserver).connection = md.reader
	cil.(*ciliumeventobserver).retryDelay = 1 * time.Millisecond
	cil.(*ciliumeventobserver).maxAttempts = 1

	go cil.Start(ctxWithCancel) //nolint:errcheck // do not need for test
	pl := getPayload()
	msg, _ := pl.Encode()
	_, _ = md.writer.Write(msg)
	event := <-exChan
	assert.Assert(t, event != nil)

	// Clean up: cancel context then close pipe to unblock monitorLoop.
	cancel()
	md.reader.Close()
	md.writer.Close()
}

func TestMonitorLoop(t *testing.T) {
	ctxWithCancel, cancel := context.WithCancel(context.Background())
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	cfg := &config.Config{
		EnablePodLevel: true,
	}
	cil := New(cfg)
	_ = cil.Init()
	cil.(*ciliumeventobserver).retryDelay = 1 * time.Millisecond
	cil.(*ciliumeventobserver).maxAttempts = 1

	// Test monitorLoop
	md := NewMockDialer(false)
	cil.(*ciliumeventobserver).d = md
	cil.(*ciliumeventobserver).connection = md.reader
	go cil.(*ciliumeventobserver).monitorLoop(ctxWithCancel) //nolint:errcheck // do not need for test

	pl := getPayload()
	msg, _ := pl.Encode()
	_, err := md.writer.Write(msg)
	assert.NilError(t, err)
	time.Sleep(2 * time.Second)
	plEvent := <-cil.(*ciliumeventobserver).payloadEvents
	assert.Assert(t, plEvent != nil)

	// Clean up: cancel context then close pipe to unblock monitorLoop.
	cancel()
	md.reader.Close()
	md.writer.Close()
}

func TestParse(t *testing.T) {
	ctxWithCancel, cancel := context.WithCancel(context.Background())
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics(slog.Default())
	cfg := &config.Config{
		EnablePodLevel: true,
	}
	cil := New(cfg)
	_ = cil.Init()
	exChannel := make(chan *v1.Event, 1)
	_ = cil.SetupChannel(exChannel)
	cil.(*ciliumeventobserver).retryDelay = 1 * time.Millisecond
	cil.(*ciliumeventobserver).maxAttempts = 1

	md := NewMockDialer(false)
	cil.(*ciliumeventobserver).d = md
	cil.(*ciliumeventobserver).connection = md.reader
	go cil.(*ciliumeventobserver).monitorLoop(ctxWithCancel) //nolint:errcheck // do not need for test

	pl := getPayload()
	msg, _ := pl.Encode()
	_, err := md.writer.Write(msg)
	assert.NilError(t, err)
	go cil.(*ciliumeventobserver).parserLoop(ctxWithCancel)
	time.Sleep(2 * time.Second)
	assert.Assert(t, len(cil.(*ciliumeventobserver).payloadEvents) == 0)
	event := <-exChannel
	assert.Assert(t, event != nil)

	// Clean up: cancel context then close pipe to unblock monitorLoop.
	cancel()
	md.reader.Close()
	md.writer.Close()
}

func getPayload() payload.Payload {
	dn := monitor.DropNotify{
		Type:    byte(monitorAPI.MessageTypeDrop),
		SubType: uint8(130),
	}

	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{1, 2, 3, 4, 5, 6},
		DstMAC:       net.HardwareAddr{6, 5, 4, 3, 2, 1},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		SrcIP:    net.IP{1, 2, 3, 4},
		DstIP:    net.IP{5, 6, 7, 8},
		Version:  4,
		Protocol: layers.IPProtocolTCP,
		TTL:      64,
	}
	tcp := &layers.TCP{
		SrcPort: 12345,
		DstPort: 80,
	}
	_ = tcp.SetNetworkLayerForChecksum(ip)

	data, _ := testutils.CreateL3L4Payload(dn, eth, ip, tcp)
	pl := payload.Payload{
		Data: data,
		CPU:  0,
		Lost: 0,
		Type: payload.EventSample,
	}
	return pl
}

type MockDialer struct {
	returnsError bool
	reader       net.Conn
	writer       net.Conn
}

func NewMockDialer(re bool) *MockDialer {
	me := &MockDialer{returnsError: re}
	if !re {
		r, w := net.Pipe()
		me.reader = r
		me.writer = w
	}
	return me
}

func (d *MockDialer) Dial(_, _ string) (net.Conn, error) {
	if d.returnsError {
		return nil, errors.New("error") //nolint:goerr113 // do not need for test
	}
	return d.reader, nil
}
