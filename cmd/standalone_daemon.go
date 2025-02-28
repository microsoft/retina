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

func NewStandaloneDaemon(config *config.Config, zl *log.ZapLogger) (*StandaloneDaemon, error) {
	fmt.Println("starting Standalone Retina daemon")
	sdLogger := zl.Named("standalone-daemon")

	var tel telemetry.Telemetry
	var err error
	if config.EnableTelemetry {
		if buildinfo.ApplicationInsightsID == "" {
			panic("telemetry enabled, but ApplicationInsightsID is empty")
		}
		sdLogger.Info("telemetry enabled", zap.String("applicationInsightsID", buildinfo.ApplicationInsightsID))
		tel, err = telemetry.NewAppInsightsTelemetryClient("standalone-retina-agent", map[string]string{
			"version": buildinfo.Version,
			"plugins": strings.Join(config.EnabledPlugin, `,`),
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
		config,
		tel,
	)
	if err != nil {
		return nil, err
	}

	// create HTTP server for API server
	httpServer := sm.NewHTTPServer(
		config.APIServer.Host,
		config.APIServer.Port,
	)

	return &StandaloneDaemon{
		l:             sdLogger,
		httpServer:    httpServer,
		pluginManager: pMgr,
		config:        config,
		tel:           tel,
	}, nil
}

func (sm *StandaloneDaemon) Start() {
	sm.l.Info("Starting standalone daemon")

	// Start the HTTP server and initialize the cache
	if err := sm.httpServer.Init(); err != nil {
		sm.l.Error("failed to start http server")
	}
	sm.cache = cache.NewStandaloneCache()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start heartbeat goroutine for application insights
	go sm.tel.Heartbeat(ctx, sm.config.TelemetryInterval)

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		return sm.pluginManager.Start(ctx)
	})
	g.Go(func() error {
		return sm.httpServer.Start(ctx)
	})

	if err := g.Wait(); err != nil {
		sm.l.Panic("Error running standalone daemon", zap.Error(err))
	}

	sm.l.Info("Started standalone daemon")
}

func (sm *StandaloneDaemon) Stop() {
	// Clean up plugin resources
	sm.pluginManager.Stop()
	sm.l.Info("Stopped the standalone daemon")
}
