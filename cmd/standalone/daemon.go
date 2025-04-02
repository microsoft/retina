// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"context"
	"fmt"

	"github.com/microsoft/retina/cmd/utils"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	pm "github.com/microsoft/retina/pkg/managers/pluginmanager"
	sm "github.com/microsoft/retina/pkg/managers/servermanager"
)

type Daemon struct {
	config        *config.Config
	httpServer    *sm.HTTPServer
	pluginManager *pm.PluginManager
}

func NewDaemon(daemonCfg *config.Config) *Daemon {
	return &Daemon{
		config: daemonCfg,
	}
}

func (d *Daemon) Start(zl *log.ZapLogger) error {
	zl.Info("Starting standalone Retina daemon")
	mainLogger := zl.Named("standalone-daemon").Sugar()

	metrics.InitializeMetrics()

	tel, err := utils.InitializeTelemetryClient(nil, d.config, mainLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := cache.NewStandaloneCache()
	enrich := enricher.NewStandaloneEnricher(ctx, cache, d.config)
	enrich.Run()

	d.pluginManager, err = pm.NewPluginManager(
		d.config,
		tel,
	)
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	d.httpServer = sm.NewHTTPServer(
		d.config.APIServer.Host,
		d.config.APIServer.Port,
	)

	if err := d.httpServer.Init(); err != nil {
		mainLogger.Fatal("Failed to start http server", zap.Error(err))
	}
	defer d.pluginManager.Stop()

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, d.config.TelemetryInterval)

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		return d.pluginManager.Start(ctx)
	})
	g.Go(func() error {
		return d.httpServer.Start(ctx)
	})

	if err := g.Wait(); err != nil {
		mainLogger.Panic("Error running standalone daemon", zap.Error(err))
	}

	mainLogger.Info("Started standalone daemon")
	return nil
}
