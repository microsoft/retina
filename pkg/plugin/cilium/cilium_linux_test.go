package cilium

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
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
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

	// Test monitorLoop
	reader, writer := net.Pipe()
	// overwrite the connection
	cil.(*cilium).connection = reader
	go cil.(*cilium).monitorLoop(ctxWithCancel)

	go func() {
		defer writer.Close()
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
	}()

	time.Sleep(5 * time.Second)
	cancel()
}
