// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package base

import (
	"context"
	"time"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher/base"
	"github.com/microsoft/retina/pkg/log"
	pm "github.com/microsoft/retina/pkg/managers/pluginmanager"
	sm "github.com/microsoft/retina/pkg/managers/servermanager"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	ResyncTime time.Duration = 5 * time.Minute
)

type Controller struct {
	L             *log.ZapLogger
	HTTPServer    *sm.HTTPServer
	PluginManager *pm.PluginManager
	Tel           telemetry.Telemetry
	Cache         cache.CacheInterface
	Enricher      base.EnricherInterface
}

func (m *Controller) Start(ctx context.Context) {
	// Only track panics if telemetry is enabled
	defer telemetry.TrackPanic()

	var g *errgroup.Group

	g, ctx = errgroup.WithContext(ctx)

	//nolint:gocritic
	// defer m.otelAgent.Start(ctx)()
	g.Go(func() error {
		return m.PluginManager.Start(ctx)
	})
	g.Go(func() error {
		return m.HTTPServer.Start(ctx)
	})
	//nolint:gocritic
	// g.Go(func() error {
	// 	return m.clusterObsCl.Start()
	// })

	if err := g.Wait(); err != nil {
		m.L.Panic("Error running controller manager", zap.Error(err))
	}
}

func (m *Controller) Stop() {
	// Stop the plugin manager. This will help clean up the plugin resources.
	m.PluginManager.Stop()
	m.L.Info("Stopped controller manager")
}
