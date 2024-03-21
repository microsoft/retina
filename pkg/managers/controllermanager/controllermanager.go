// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package controllermanager

import (
	"context"
	"errors"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	pm "github.com/microsoft/retina/pkg/managers/pluginmanager"
	sm "github.com/microsoft/retina/pkg/managers/servermanager"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/microsoft/retina/pkg/track"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

const (
	ResyncTime     time.Duration = 5 * time.Minute
	errFailedTrack               = "Failed to create track instance"
)

type Controller struct {
	l             *log.ZapLogger
	httpServer    *sm.HTTPServer
	pluginManager *pm.PluginManager
	tel           telemetry.Telemetry
	conf          *kcfg.Config
	pubsub        *pubsub.PubSub
	cache         *cache.Cache
	enricher      *enricher.Enricher
	t             *track.Track
}

func NewControllerManager(conf *kcfg.Config, kubeclient kubernetes.Interface, tel telemetry.Telemetry) (*Controller, error) {
	cmLogger := log.Logger().Named("controller-manager")

	if conf.EnablePodLevel {
		// informer factory for pods/services
		factory := informers.NewSharedInformerFactory(kubeclient, ResyncTime)
		factory.WaitForCacheSync(wait.NeverStop)
	}

	// enabledPlugins := {api.PluginName(conf.EnabledPlugin[])}
	enabledPlugins := []api.PluginName{}
	for _, pluginName := range conf.EnabledPlugin {
		enabledPlugins = append(enabledPlugins, api.PluginName(pluginName))
	}
	pMgr, err := pm.NewPluginManager(
		conf,
		tel,
		enabledPlugins...,
	)
	if err != nil {
		return nil, err
	}

	// create HTTP server for API server
	httpServer := sm.NewHTTPServer(
		conf.ApiServer.Host,
		conf.ApiServer.Port,
	)

	return &Controller{
		l:             cmLogger,
		httpServer:    httpServer,
		pluginManager: pMgr,
		tel:           tel,
		conf:          conf,
	}, nil
}

func (m *Controller) Init(ctx context.Context) error {
	m.l.Info("Initializing controller manager ...")

	if err := m.httpServer.Init(); err != nil {
		return err
	}

	if m.conf.EnablePodLevel {
		// create pubsub instance
		m.pubsub = pubsub.New()

		// create cache instance
		m.cache = cache.New(m.pubsub)

		// create enricher instance
		m.enricher = enricher.New(ctx, m.cache)

		// create track instance
		m.t = track.New()
		if m.t == nil {
			return errors.New(errFailedTrack)
		}
		// Setup with plugins.
		m.pluginManager.SetupChannel(m.t.Channel())
	}

	return nil
}

func (m *Controller) Start(ctx context.Context) {
	// Only track panics if telemetry is enabled
	defer telemetry.TrackPanic()

	var g *errgroup.Group

	g, ctx = errgroup.WithContext(ctx)

	if m.conf.EnablePodLevel {
		// Start tracking events.
		// Check is necessary because tracking is useless without pod level enabled.
		if m.t != nil {
			g.Go(func() error {
				m.t.Start(ctx)
				return nil
			})
		}
	}

	// defer m.otelAgent.Start(ctx)()
	g.Go(func() error {
		return m.pluginManager.Start(ctx)
	})
	g.Go(func() error {
		return m.httpServer.Start(ctx)
	})
	// g.Go(func() error {
	// 	return m.clusterObsCl.Start()
	// })

	if err := g.Wait(); err != nil {
		m.l.Panic("Error running controller manager", zap.Error(err))
	}
}

func (m *Controller) Stop(ctx context.Context) {
	// Stop the plugin manager. This will help clean up the plugin resources.
	m.pluginManager.Stop()
	// Stop tracking events.
	if m.t != nil {
		m.t.Stop()
	}
	m.l.Info("Stopped controller manager")
}
