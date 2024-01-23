// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"github.com/microsoft/retina/pkg/bpf"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
)

var (
	applicationInsightsID string //nolint // aiMetadata is set in Makefile
	version               string
)

func main() {
	// Initialize application insights
	telemetry.InitAppInsights(applicationInsightsID, version)
	defer telemetry.TrackPanic()

	// Initialize logger
	opts := log.GetDefaultLogOpts()
	opts.ApplicationInsightsID = applicationInsightsID
	opts.EnableTelemetry = true

	log.SetupZapLogger(opts)
	log.Logger().AddFields(zap.String("version", version))
	l := log.Logger().Named("init-retina")

	// Setup BPF
	bpf.Setup(l)
}
