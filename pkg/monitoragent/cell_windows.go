// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package monitoragent

import (
	"context"

	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	ciliumagent "github.com/cilium/cilium/pkg/monitor/agent"
	"github.com/cilium/cilium/pkg/monitor/agent/consumer"
	"github.com/cilium/cilium/pkg/monitor/agent/listener"
	"github.com/cilium/hive/cell"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	Cell = cell.Module(
		"monitor-agent",
		"Consumes the Windows events and distributes those and other agent events",

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
		listeners:        make(map[listener.MonitorListener]struct{}),
		consumers:        make(map[consumer.MonitorConsumer]struct{}),
		perfReaderCancel: func() {}, // no-op to avoid doing null checks everywhere
	}

	params.Lifecycle.Append(cell.Hook{
		OnStart: func(cell.HookContext) error {
			// On Windows, we don't automatically attach to the EVENTS_MAP on start
			// because only one consumer can attach at a time. The attachment should
			// be explicitly requested, and any failure should be visible.
			//
			// Note: When AttachToEventsMap is called and fails, it will return
			// ErrEventsMapAttachFailed to make the failure visible rather than
			// silently succeeding.
			params.Log.Info("Monitor agent started (Windows)")
			return nil
		},
		OnStop: func(cell.HookContext) error {
			cancel()
			return nil
		},
	})

	return agent
}
