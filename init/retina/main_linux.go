// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/bpf"
	"github.com/microsoft/retina/pkg/ciliumfs"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func main() {
	if err := run(os.Args[1:]...); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(args ...string) error {
	// Initialize logger
	opts := log.GetDefaultLogOpts()
	zl, err := log.SetupZapLogger(opts)
	if err != nil {
		return errors.Wrap(err, "failed to setup logger")
	}
	l := zl.Named("init-retina").With(zap.String("version", buildinfo.Version))

	// Parse flags from args
	fs := flag.NewFlagSet("retina", flag.ContinueOnError)
	configPath := fs.String("config", "/retina/config/config.yaml", "path to the config file")
	err = fs.Parse(args)
	if err != nil {
		return errors.Wrap(err, "failed to parse flags")
	}

	// Get config
	cfg, err := config.GetConfig(*configPath)
	if err != nil {
		return errors.Wrap(err, "failed to get config")
	}

	// Enable telemetry if applicationInsightsID is provided and telemetry is enabled in config.
	if buildinfo.ApplicationInsightsID != "" && cfg.EnableTelemetry {
		opts.EnableTelemetry = true
		opts.ApplicationInsightsID = buildinfo.ApplicationInsightsID
		// Initialize application insights
		telemetry.InitAppInsights(buildinfo.ApplicationInsightsID, buildinfo.Version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()
	}

	// Setup BPF
	err = bpf.Setup(l)
	if err != nil {
		return errors.Wrap(err, "failed to setup Retina bpf filesystem")
	}

	// Setup CiliumFS.
	err = ciliumfs.Setup(l)
	if err != nil {
		return errors.Wrap(err, "failed to setup CiliumFS")
	}

	return nil
}
