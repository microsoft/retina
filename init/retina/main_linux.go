// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"github.com/microsoft/retina/pkg/bpf"
	"github.com/microsoft/retina/pkg/ciliumfs"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
)

var (
	// applicationInsightsID is the instrumentation key for Azure Application Insights
	// It is set during the build process using the -ldflags flag
	// If it is set, the application will send telemetry to the corresponding Application Insights resource.
	applicationInsightsID string
	version               string
)

func main() {
	// Initialize logger
	opts := log.GetDefaultLogOpts()

	// Enable telemetry if applicationInsightsID is provided
	if applicationInsightsID != "" {
		opts.EnableTelemetry = true
		opts.ApplicationInsightsID = applicationInsightsID
		// Initialize application insights
		telemetry.InitAppInsights(applicationInsightsID, version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()
	}

	zl, err := log.SetupZapLogger(opts)
	if err != nil {
		panic(err)
	}
	l := zl.Named("init-retina").With(zap.String("version", version))

	// Setup BPF
	bpf.Setup(l)

	// Setup CiliumFS.
	ciliumfs.Setup(l)
}
