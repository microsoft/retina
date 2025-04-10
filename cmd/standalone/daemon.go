// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"fmt"
	"sync"

	"github.com/microsoft/retina/cmd/utils"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	cm "github.com/microsoft/retina/pkg/managers/controllermanager"
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

	tel, err := utils.InitializeTelemetryClient(nil, d.config, mainLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry client: %w", err)
	}

	ctx := ctrl.SetupSignalHandler() // Importing K8s signal handler - ok?

	cache := cache.NewStandaloneCache()
	enrich := enricher.NewStandaloneEnricher(ctx, cache, d.config)
	enrich.Run()

	// enable pod level needs to be false!
	controllerMgr, err := cm.NewControllerManager(d.config, nil, tel)
	if err != nil {
		mainLogger.Fatal("Failed to create controller manager", zap.Error(err))
	}
	if err := controllerMgr.Init(ctx); err != nil {
		mainLogger.Fatal("Failed to initialize controller manager", zap.Error(err))
	}
	defer controllerMgr.Stop(ctx)

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, d.config.TelemetryInterval)

	// Start controller manager, which will start the http server and plugin manager
	go controllerMgr.Start(ctx)
	mainLogger.Info("Started controller manager")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
	}()

	wg.Wait()

	mainLogger.Info("Network observability exiting. Till next time!")
	return nil
}
