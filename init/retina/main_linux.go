// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"flag"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/bpf"
	"github.com/microsoft/retina/pkg/ciliumfs"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	opts := log.GetDefaultLogOpts()
	zl, err := log.SetupZapLogger(opts)
	if err != nil {
		panic(err)
	}
	l := zl.Named("init-retina").With(zap.String("version", buildinfo.Version))

	configPath := flag.String("config", "/retina/config/config.yaml", "path to the config file")
	cfg, err := config.GetConfig(*configPath)
	if err != nil {
		l.Fatal("failed to get config", zap.Error(err))
	}

	// Enable telemetry if applicationInsightsID is provided and telemetry is enabled
	if buildinfo.ApplicationInsightsID != "" && cfg.EnableTelemetry {
		opts.EnableTelemetry = true
		opts.ApplicationInsightsID = buildinfo.ApplicationInsightsID
		// Initialize application insights
		telemetry.InitAppInsights(buildinfo.ApplicationInsightsID, buildinfo.Version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()
	}

	// Setup BPF
	bpf.Setup(l)

	// Setup CiliumFS.
	ciliumfs.Setup(l)
}
