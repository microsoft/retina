package ciliumeventobserver

//nolint:typecheck // do not need for test
import (
	"context"
	"net"
	"testing"
	"time"

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

func TestStart(t *testing.T) {
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

	err := cil.Start(ctxTimeout)
	assert.Assert(t, err != nil, "Error should not be nil")
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
	reader, writer := net.Pipe()
	defer reader.Close()
	defer writer.Close()
	// overwrite the connection
	cil.(*ciliumeventobserver).connection = reader
	go cil.(*ciliumeventobserver).monitorLoop(ctxWithCancel)

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
	msg, _ := pl.Encode()
	_, err := writer.Write(msg)
	assert.NilError(t, err)
	cancel()
}

func TestMonitorLoopWithRetry(t *testing.T) {
	ctxWithCancel, cancel := context.WithCancel(context.Background())
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	cfg := &config.Config{
		EnablePodLevel: true,
	}
	cil := New(cfg)
	_ = cil.Init()
	cil.(*ciliumeventobserver).retryDelay = 1 * time.Millisecond
	cil.(*ciliumeventobserver).maxAttempts = 2
	// Test monitorLoop
	reader, writer := net.Pipe()
	cil.(*ciliumeventobserver).connection = reader
	go cil.(*ciliumeventobserver).monitorLoop(ctxWithCancel)
	err := writer.Close()
	assert.NilError(t, err)
	time.Sleep(5 * time.Second)
	assert.Assert(t, cil.(*ciliumeventobserver).connection == nil, "connection should be nil")
	cancel()
}
