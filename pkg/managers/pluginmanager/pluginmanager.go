// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package pluginmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/watchermanager"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (

	// In any run I haven't seen reconcile take longer than 5 seconds,
	// and 10 seconds seems like a reasonable SLA for reconciliation to be completed
	MAX_RECONCILE_TIME = 10 * time.Second
)

const (
	MAX_STARTUP_TIME = 10 * time.Second
)

var (
	ErrNilCfg       = errors.New("pluginmanager requires a non-nil config")
	ErrZeroInterval = errors.New("pluginmanager requires a positive MetricsInterval in its config")
)

type PluginManager struct {
	cfg     *kcfg.Config
	l       *log.ZapLogger
	plugins map[api.PluginName]api.Plugin
	tel     telemetry.Telemetry

	watcherManager watchermanager.Manager
}

func init() {
	registry.RegisterPlugins()
}

func NewPluginManager(
	cfg *kcfg.Config,
	tel telemetry.Telemetry,
	pluginNames ...api.PluginName,
) (*PluginManager, error) {
	logger := log.Logger().Named("plugin-manager")
	mgr := &PluginManager{
		cfg:     cfg,
		l:       logger,
		tel:     tel,
		plugins: map[api.PluginName]api.Plugin{},
	}

	if mgr.cfg.EnablePodLevel {
		mgr.l.Info("plugin manager has pod level enabled")
		mgr.watcherManager = watchermanager.NewWatcherManager()
	} else {
		mgr.l.Info("plugin manager has pod level disabled")
	}

	for _, name := range pluginNames {
		newPluginFn, ok := registry.PluginHandler[name]
		if !ok {
			return nil, fmt.Errorf("plugin %s not found in registry", name)
		}
		mgr.plugins[name] = newPluginFn(mgr.cfg)
	}

	return mgr, nil
}

func (p *PluginManager) Stop() {
	var wg sync.WaitGroup
	for _, plugin := range p.plugins {
		wg.Add(1)
		go func(plugin api.Plugin) {
			defer wg.Done()
			if err := plugin.Stop(); err != nil {
				p.l.Error("failed to stop plugin", zap.Error(err))
				// Continue stopping other plugins.
				// This allows us to stop as many plugins as possible,
				// even if some plugins fail to stop.
			}
			p.l.Info("Cleaned up resource for plugin", zap.String("name", plugin.Name()))
		}(plugin)
	}
	wg.Wait()
}

// Reconcile reconciles a particular plugin.
func (p *PluginManager) Reconcile(ctx context.Context, plugin api.Plugin) error {
	defer p.tel.StopPerf(p.tel.StartPerf(fmt.Sprintf("reconcile-%s", plugin.Name())))
	// Regenerate eBPF code and bpf object.
	// This maybe no-op for plugins that don't use eBPF.
	if err := plugin.Generate(ctx); err != nil {
		return err
	}
	if err := plugin.Compile(ctx); err != nil {
		return err
	}

	// Re-start plugin.
	if err := plugin.Stop(); err != nil {
		return err
	}
	if err := plugin.Init(); err != nil {
		return err
	}

	p.l.Info("Reconciled plugin", zap.String("name", plugin.Name()))
	return nil
}

// Start plugin manager.
// Note: This will block as long as main thread is running.
func (p *PluginManager) Start(ctx context.Context) error {
	counter := p.tel.StartPerf("start-plugin-manager")
	// start plugins evenly throughout the interval,
	// if 2 plugins enabled, and 10 second interval
	// 10 / 2 = 5, then after every start sleep 5s
	// plugin 1 = 0s
	// plugin 2 = 5s
	// then the plugins won't awake at the same time and they'll have even execution time

	delay := float32(MAX_STARTUP_TIME) / float32(len(p.plugins))
	p.l.Info("Starting plugin manager ...")
	var err error

	if p.cfg == nil {
		return ErrNilCfg
	}

	if p.cfg.MetricsInterval == 0 {
		return ErrZeroInterval
	}

	g, ctx := errgroup.WithContext(ctx)
	if p.cfg.EnablePodLevel {
		// Start watcher manager
		g.Go(func() error {
			return p.watcherManager.Start(ctx)
		})
	}
	// start all plugins
	for _, plugin := range p.plugins {
		plug := plugin

		reconcilectx, cancel := context.WithTimeout(ctx, time.Duration(MAX_RECONCILE_TIME))
		defer cancel()
		err = p.Reconcile(reconcilectx, plug)
		if err != nil {
			// Update control plane metrics counter
			metrics.PluginManagerFailedToReconcileCounter.WithLabelValues(plugin.Name()).Inc()
			return errors.Wrapf(err, "failed to reconcile plugin %s", plugin.Name())
		}

		g.Go(func() error {
			p.l.Info(fmt.Sprintf("starting plugin %s", plug.Name()))
			return errors.Wrapf(plug.Start(ctx), "failed to start plugin %s", plug.Name())
		})

		time.Sleep(time.Duration(delay))
	}

	p.tel.StopPerf(counter)
	p.l.Info("successfully started pluginmanager")
	// on cancel context wait for all plugins to exit
	err = g.Wait()
	if err != nil {
		p.l.Error("plugin manager exited with error", zap.Error(err))
		return errors.Wrapf(err, "failed to start plugin manager, plugin exited")
	}

	if p.cfg.EnablePodLevel {
		p.l.Info("stopping watcher manager")

		// Stop watcher manager.
		if err := p.watcherManager.Stop(ctx); err != nil {
			return errors.Wrap(err, "failed to stop watcher manager")
		}
	}

	p.l.Info("stopping pluginmanager...")
	return nil
}

func (p *PluginManager) SetPlugin(name api.PluginName, plugin api.Plugin) {
	if p == nil {
		return
	}

	if p.plugins == nil {
		p.plugins = map[api.PluginName]api.Plugin{}
	}
	p.plugins[name] = plugin
}

func (p *PluginManager) SetupChannel(c chan *v1.Event) {
	for name, plugin := range p.plugins {
		err := plugin.SetupChannel(c)
		if err != nil {
			p.l.Error("failed to setup channel for plugin", zap.String("plugin name", string(name)), zap.Error(err))
		}
	}
}
