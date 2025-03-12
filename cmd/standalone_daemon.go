// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"context"
	"fmt"

	"github.com/microsoft/retina/cmd/telemetry"
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

type StandaloneDaemon struct {
	config        *config.Config
	httpServer    *sm.HTTPServer
	pluginManager *pm.PluginManager
}

func NewStandaloneDaemon(daemonCfg *config.Config) *StandaloneDaemon {
	return &StandaloneDaemon{
		config: daemonCfg,
	}
}

func (sd *StandaloneDaemon) Start(zl *log.ZapLogger) error {
	fmt.Println("Starting standalone Retina daemon")
	mainLogger := zl.Named("standalone-daemon").Sugar()

	metrics.InitializeMetrics()

	tel, err := telemetry.InitializeTelemetryClient(nil, sd.config, mainLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := cache.NewStandaloneCache()
	enrich := enricher.NewStandaloneEnricher(ctx, cache)
	enrich.Run()

	sd.pluginManager, err = pm.NewPluginManager(
		sd.config,
		tel,
	)
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	sd.httpServer = sm.NewHTTPServer(
		sd.config.APIServer.Host,
		sd.config.APIServer.Port,
	)

	if err := sd.httpServer.Init(); err != nil {
		mainLogger.Fatal("Failed to start http server", zap.Error(err))
	}
	defer sd.pluginManager.Stop()

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, sd.config.TelemetryInterval)

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		return sd.pluginManager.Start(ctx)
	})
	g.Go(func() error {
		return sd.httpServer.Start(ctx)
	})

	if err := g.Wait(); err != nil {
		mainLogger.Panic("Error running standalone daemon", zap.Error(err))
	}

	mainLogger.Info("Started standalone daemon")
	return nil
}
