package pluginmanager

import (
	"context"
	"log/slog"
	"os"
	"sync"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/telemetry"
)

const (
	// Default external channel size for events
	// This is the default size of the channel that is used to send events from plugins to hubble
	DefaultExternalEventChannelSize = 10000
)

var Cell = cell.Module(
	"pluginmanager",
	"Manages Retina eBPF plugins",
	cell.Provide(func() chan *v1.Event {
		return make(chan *v1.Event, DefaultExternalEventChannelSize)
	}),
	cell.Provide(newPluginManager),
)

type pluginManagerParams struct {
	cell.In

	Log       *slog.Logger
	Lifecycle cell.Lifecycle
	Config    config.Config
	Telemetry telemetry.Telemetry
	EventChan chan *v1.Event
}

func newPluginManager(params pluginManagerParams) (*PluginManager, error) {
	logger := params.Log.With("module", "pluginmanager")

	// Enable Metrics in retina
	metrics.InitializeMetrics(params.Log)

	pluginMgr, err := NewPluginManager(&params.Config, params.Telemetry, params.Log)
	if err != nil {
		return &PluginManager{}, err
	}

	pmCtx, cancelCtx := context.WithCancel(context.Background())
	// Setup the event channel to be used by hubble
	pluginMgr.SetupChannel(params.EventChan)

	var wg sync.WaitGroup
	params.Lifecycle.Append(cell.Hook{
		OnStart: func(cell.HookContext) error {
			var err error
			wg.Add(1)
			go func() {
				defer wg.Done()
				err = pluginMgr.Start(pmCtx)
				if err != nil {
					logger.Error("failed to start plugin manager", "error", err)
					os.Exit(1)
				}
			}()

			return err
		},
		OnStop: func(cell.HookContext) error {
			cancelCtx()
			pluginMgr.Stop()
			wg.Wait()
			return nil
		},
	})
	return pluginMgr, nil
}
