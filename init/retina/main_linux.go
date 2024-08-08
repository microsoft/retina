// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/bpf"
	"github.com/microsoft/retina/pkg/ciliumfs"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	opts := log.GetDefaultLogOpts()

	// Enable telemetry if applicationInsightsID is provided
	if buildinfo.ApplicationInsightsID != "" {
		opts.EnableTelemetry = true
		opts.ApplicationInsightsID = buildinfo.ApplicationInsightsID
		// Initialize application insights
		telemetry.InitAppInsights(buildinfo.ApplicationInsightsID, buildinfo.Version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()
	}

	zl, err := log.SetupZapLogger(opts)
	if err != nil {
		panic(err)
	}
	l := zl.Named("init-retina").With(zap.String("version", buildinfo.Version))

	// Setup BPF
	bpf.Setup(l)

	// Setup CiliumFS.
	ciliumfs.Setup(l)
}
