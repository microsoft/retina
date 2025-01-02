package pluginmanager

import (
	"context"
	"sync"

	"github.com/cilium/hive/cell"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/sirupsen/logrus"
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

	Log       logrus.FieldLogger
	Lifecycle cell.Lifecycle
	Config    config.Config
	Telemetry telemetry.Telemetry
	EventChan chan *v1.Event
}

func newPluginManager(params pluginManagerParams) (*PluginManager, error) {
	logger := params.Log.WithField("module", "pluginmanager")

	// Enable Metrics in retina
	metrics.InitializeMetrics()

	pluginMgr, err := NewPluginManager(&params.Config, params.Telemetry)
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
					logger.WithError(err).Fatal("failed to start plugin manager")
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
