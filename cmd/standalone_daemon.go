// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	pm "github.com/microsoft/retina/pkg/managers/pluginmanager"
	sm "github.com/microsoft/retina/pkg/managers/servermanager"
)

type StandaloneDaemon struct {
	l             *log.ZapLogger
	cache         *cache.StandaloneCache
	httpServer    *sm.HTTPServer
	pluginManager *pm.PluginManager
	config        *config.Config
	tel           telemetry.Telemetry
}

func NewStandaloneDaemon(cfg *config.Config, zl *log.ZapLogger) (*StandaloneDaemon, error) {
	fmt.Println("starting Standalone Retina daemon")
	sdLogger := zl.Named("standalone-daemon")

	var tel telemetry.Telemetry
	var err error
	if cfg.EnableTelemetry {
		if buildinfo.ApplicationInsightsID == "" {
			panic("telemetry enabled, but ApplicationInsightsID is empty")
		}
		sdLogger.Info("telemetry enabled", zap.String("applicationInsightsID", buildinfo.ApplicationInsightsID))
		tel, err = telemetry.NewAppInsightsTelemetryClient("standalone-retina-agent", map[string]string{
			"version": buildinfo.Version,
			"plugins": strings.Join(cfg.EnabledPlugin, `,`),
		})
		if err != nil {
			sdLogger.Error("failed to create telemetry client", zap.Error(err))
			return nil, fmt.Errorf("error when creating telemetry client: %w", err)
		}
	} else {
		sdLogger.Info("telemetry disabled")
		tel = telemetry.NewNoopTelemetry()
	}

	pMgr, err := pm.NewPluginManager(
		cfg,
		tel,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin manager: %w", err)
	}

	// create HTTP server for API server
	httpServer := sm.NewHTTPServer(
		cfg.APIServer.Host,
		cfg.APIServer.Port,
	)

	return &StandaloneDaemon{
		l:             sdLogger,
		httpServer:    httpServer,
		pluginManager: pMgr,
		config:        cfg,
		tel:           tel,
	}, nil
}

func (sd *StandaloneDaemon) Start() {
	sd.l.Info("Starting standalone daemon")

	// Start the HTTP server and initialize the cache
	if err := sd.httpServer.Init(); err != nil {
		sd.l.Error("failed to start http server")
	}
	sd.cache = cache.NewStandaloneCache()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start heartbeat goroutine for application insights
	go sd.tel.Heartbeat(ctx, sd.config.TelemetryInterval)

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		return sd.pluginManager.Start(ctx)
	})
	g.Go(func() error {
		return sd.httpServer.Start(ctx)
	})

	if err := g.Wait(); err != nil {
		sd.l.Panic("Error running standalone daemon", zap.Error(err))
	}

	sd.l.Info("Started standalone daemon")
}

func (sd *StandaloneDaemon) Stop() {
	// Clean up plugin resources
	sd.pluginManager.Stop()
	sd.l.Info("Stopped the standalone daemon")
}
