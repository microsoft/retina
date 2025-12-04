// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"fmt"

	"github.com/microsoft/retina/cmd/observability"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/metrics"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/microsoft/retina/pkg/config"
	cache "github.com/microsoft/retina/pkg/controllers/cache/standalone"
	sc "github.com/microsoft/retina/pkg/controllers/daemon/standalone"
	cm "github.com/microsoft/retina/pkg/managers/controllermanager"
	sm "github.com/microsoft/retina/pkg/module/metrics/standalone"
)

type Daemon struct {
	configFile string
}

func NewDaemon(configFile string) *Daemon {
	return &Daemon{
		configFile: configFile,
	}
}

func (d *Daemon) Start() error {
	fmt.Printf("Starting Retina daemon in standalone mode\n")

	daemonCfg, err := config.GetStandaloneConfig(d.configFile)
	if err != nil {
		panic(err)
	}
	zl := observability.InitializeLogger(daemonCfg.LogLevel, daemonCfg.EnableTelemetry, daemonCfg.EnabledPlugin, daemonCfg.DataAggregationLevel)
	mainLogger := zl.Named("main").Sugar()

	// Initialize basic metrics and telemetry client
	metrics.InitializeMetrics()
	tel, err := observability.InitializeTelemetryClient(nil, daemonCfg.EnabledPlugin, daemonCfg.EnableTelemetry, mainLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry client: %w", err)
	}

	// Initialize cache and run enricher
	ctx := ctrl.SetupSignalHandler()
	controllerCache := cache.New()
	enrich := enricher.NewStandalone(ctx, controllerCache)
	enrich.Run()

	// Initialize metrics module
	metricsModule := sm.InitModule(ctx, enrich)

	mainLogger.Info("Initializing RetinaEndpoint controller")
	controller, err := sc.New(daemonCfg, controllerCache, metricsModule)
	if err != nil {
		mainLogger.Fatal("failed to create RetinaEndpoint controller", zap.Error(err))
	}
	go controller.Run(ctx)

	// Initialize controller manager
	controllerMgr, err := cm.NewStandaloneControllerManager(daemonCfg, tel)
	if err != nil {
		mainLogger.Fatal("failed to create standalone controller manager", zap.Error(err))
	}
	if err := controllerMgr.Init(); err != nil {
		mainLogger.Fatal("failed to initialize standalone controller manager", zap.Error(err))
	}

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, daemonCfg.TelemetryInterval)

	// Start controller manager, which will start the http server and plugin manager
	go controllerMgr.Start(ctx)
	mainLogger.Info("Started controller manager")

	<-ctx.Done()
	controllerMgr.Stop()

	mainLogger.Info("Network observability exiting. Till next time!")
	return nil
}
