package main

import (
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/microsoft/retina/pkg/capture"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
)

var (
	applicationInsightsID string //nolint // aiMetadata is set in Makefile
	version               string //nolint
)

func main() {
	lOpts := log.GetDefaultLogOpts()
	lOpts.ApplicationInsightsID = applicationInsightsID
	log.SetupZapLogger(lOpts)
	l := log.Logger().Named("captureworkload")
	l.Info("Start to capture network traffic")
	l.Info("Version: ", zap.String("version", version))
	l.Info("Start telemetry with App Insights ID: ", zap.String("applicationInsightsID", applicationInsightsID))

	telemetry.InitAppInsights(applicationInsightsID, version)
	defer telemetry.ShutdownAppInsights()

	// Create channel to listen for signals.
	sigChan := make(chan os.Signal, 1)
	// Notify sigChan for SIGTERM.
	signal.Notify(sigChan, syscall.SIGTERM)

	tel := telemetry.NewAppInsightsTelemetryClient("retina-capture", map[string]string{
		"version":                   version,
		telemetry.PropertyApiserver: os.Getenv(captureConstants.ApiserverEnvKey),
	})

	cm := capture.NewCaptureManager(l, tel)

	defer func() {
		if err := cm.Cleanup(); err != nil {
			l.Error("Failed to cleanup network capture", zap.Error(err))
		}
	}()
	srcDir, err := cm.CaptureNetwork(sigChan)
	if err != nil {
		l.Error("Failed to capture network traffic", zap.Error(err))
		os.Exit(1)
	}
	if err := cm.OutputCapture(srcDir); err != nil {
		l.Error("Failed to output network traffic", zap.Error(err))
	}
	l.Info("Done for capturing network traffic")
}
