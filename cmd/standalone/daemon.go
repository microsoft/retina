// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"fmt"

	"github.com/microsoft/retina/cmd/telemetry"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/microsoft/retina/pkg/config"
	cache "github.com/microsoft/retina/pkg/controllers/cache/standalone"
	cm "github.com/microsoft/retina/pkg/managers/controllermanager"
)

type Daemon struct {
	config *config.Config
}

func NewDaemon(daemonCfg *config.Config) *Daemon {
	return &Daemon{
		config: daemonCfg,
	}
}

func (d *Daemon) Start(zl *log.ZapLogger) error {
	zl.Info("Starting Retina daemon in standalone mode")
	mainLogger := zl.Named("main").Sugar()

	// Initialize basic metrics and telemetry client
	metrics.InitializeMetrics()
	tel, err := telemetry.InitializeTelemetryClient(nil, d.config, mainLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry client: %w", err)
	}

	// Initialize cache and run enricher
	ctx := ctrl.SetupSignalHandler()
	controllerCache := cache.New()
	enrich := enricher.New(ctx, controllerCache, d.config.EnableStandalone)
	enrich.Run()

	// Initialize metrics module
	// nolint:gocritic
	// metricsModule := sm.InitModule(ctx, enrich)

	mainLogger.Info("Initializing RetinaEndpoint controller")
	// nolint:gocritic
	// controller := sc.New(d.config, controllerCache, metricsModule)
	// go controller.Run(ctx)

	// Standalone requires pod level to be disabled
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
