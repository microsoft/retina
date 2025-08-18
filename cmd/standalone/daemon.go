// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"fmt"
	"time"

	"github.com/microsoft/retina/cmd/telemetry"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	cm "github.com/microsoft/retina/pkg/managers/controllermanager"
)

const TTL = 3 * time.Minute

type Daemon struct {
	config *config.Config
}

func NewDaemon(daemonCfg *config.Config) *Daemon {
	return &Daemon{
		config: daemonCfg,
	}
}

func (d *Daemon) Start(zl *log.ZapLogger) error {
	zl.Info("Starting Standalone Retina daemon")
	mainLogger := zl.Named("standalone-daemon").Sugar()

	tel, err := telemetry.InitializeTelemetryClient(nil, d.config, mainLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry client: %w", err)
	}

	ctx := ctrl.SetupSignalHandler()

	c := cache.NewStandaloneCache(TTL)
	enrich := enricher.NewStandaloneEnricher(ctx, c, d.config)
	enrich.Run()

	// pod level needs to be disabled
	controllerMgr, err := cm.NewControllerManager(d.config, nil, tel)
	if err != nil {
		mainLogger.Fatal("Failed to create controller manager", zap.Error(err))
	}
	if err := controllerMgr.Init(ctx); err != nil {
		mainLogger.Fatal("Failed to initialize controller manager", zap.Error(err))
	}

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, d.config.TelemetryInterval)

	// Start controller manager, which will start the http server and plugin manager
	go controllerMgr.Start(ctx)
	mainLogger.Info("Started controller manager")

	<-ctx.Done()
	controllerMgr.Stop(ctx)

	mainLogger.Info("Network observability exiting. Till next time!")
	return nil
}
