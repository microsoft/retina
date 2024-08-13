package ciliumeventobserver

//nolint:typecheck // do not need for test
import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/testutils"
	"github.com/cilium/cilium/pkg/monitor"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/monitor/payload"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
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
	defer cancel()
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())

	cfg := &config.Config{
		EnablePodLevel: true,
	}
	cil := New(cfg)
	exChan := make(chan *v1.Event)
	_ = cil.SetupChannel(exChan)
	_ = cil.Init()
	md := NewMockDialer(false)
	cil.(*ciliumeventobserver).d = md
	cil.(*ciliumeventobserver).connection = md.reader

	go cil.Start(ctxWithCancel) //nolint:errcheck // do not need for test
	pl := getPayload()
	msg, _ := pl.Encode()
	_, _ = md.writer.Write(msg)
	event := <-exChan
	assert.Assert(t, event != nil)
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
	cancel()
}

func TestParse(t *testing.T) {
	ctxWithCancel, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	cfg := &config.Config{
		EnablePodLevel: true,
	}
	cil := New(cfg)
	_ = cil.Init()
	exChannel := make(chan *v1.Event)
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
}

func getPayload() payload.Payload {
	dn := monitor.DropNotify{
		Type:    byte(monitorAPI.MessageTypeDrop),
		SubType: uint8(130),
	}

	data, _ := testutils.CreateL3L4Payload(dn)
	pl := payload.Payload{
		Data: data,
		CPU:  0,
		Lost: 0,
		Type: 9,
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
