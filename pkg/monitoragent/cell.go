package monitoragent

import (
	"context"
	"fmt"

	"github.com/cilium/cilium/pkg/common"
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	ciliumagent "github.com/cilium/cilium/pkg/monitor/agent"
	"github.com/cilium/cilium/pkg/monitor/agent/consumer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	Cell = cell.Module(
		"monitor-agent",
		"Consumes the cilium events map and distributes those and other agent events",

		cell.Provide(newMonitorAgent),
		cell.Config(defaultConfig),
	)

	log = logging.DefaultLogger.WithField(logfields.LogSubsys, "monitor-agent")
)

type AgentConfig struct {
	// EnableMonitor enables the monitor unix domain socket server
	EnableMonitor bool

	// MonitorQueueSize is the size of the monitor event queue
	MonitorQueueSize int
}

var defaultConfig = AgentConfig{
	EnableMonitor: true,
}

func (def AgentConfig) Flags(flags *pflag.FlagSet) {
	flags.Bool("enable-monitor", def.EnableMonitor, "Enable the monitor unix domain socket server")
	flags.Int("monitor-queue-size", 0, "Size of the event queue when reading monitor events")
}

type agentParams struct {
	cell.In

	Lifecycle cell.Lifecycle
	Log       logrus.FieldLogger
	Config    AgentConfig
}

func newMonitorAgent(params agentParams) ciliumagent.Agent {
	ctx, cancel := context.WithCancel(context.Background())
	agent := &monitorAgent{
		ctx:              ctx,
		listeners:        make(map[EnqueuerCloser]struct{}),
		consumers:        make(map[consumer.MonitorConsumer]struct{}),
		perfReaderCancel: func() {}, // no-op to avoid doing null checks everywhere
	}

	params.Lifecycle.Append(cell.Hook{
		OnStart: func(cell.HookContext) error {
			var err error
			if params.Config.EnableMonitor {
				queueSize := params.Config.MonitorQueueSize
				if queueSize == 0 {
					queueSize = common.GetNumPossibleCPUs(log) * defaults.MonitorQueueSizePerCPU
					if queueSize > defaults.MonitorQueueSizePerCPUMaximum {
						queueSize = defaults.MonitorQueueSizePerCPUMaximum
					}
				}

				monitorErr := ciliumagent.ServeMonitorAPI(ctx, agent, queueSize)
				if monitorErr != nil {
					log.WithError(monitorErr).Error("encountered error serving monitor agent API")
					return fmt.Errorf("encountered error serving monitor agent API: %w", monitorErr)
				}
			}
			return err
		},
		OnStop: func(cell.HookContext) error {
			cancel()
			return nil
		},
	})

	return agent
}
