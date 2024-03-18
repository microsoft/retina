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
	// applicationInsightsID is the instrumentation key for Azure Application Insights
	// It is set during the build process using the -ldflags flag
	// If it is set, the application will send telemetry to the corresponding Application Insights resource.
	applicationInsightsID string //nolint
	version               string //nolint
)

func main() {
	lOpts := log.GetDefaultLogOpts()
	// Set Azure application insights ID if it is provided
	if applicationInsightsID != "" {
		lOpts.ApplicationInsightsID = applicationInsightsID
	}

	log.SetupZapLogger(lOpts)
	l := log.Logger().Named("captureworkload")
	l.Info("Start to capture network traffic")
	l.Info("Version: ", zap.String("version", version))

	var tel telemetry.Telemetry
	if applicationInsightsID != "" {
		l.Info("Start telemetry with Azure App Insights Monitor", zap.String("applicationInsightsID", applicationInsightsID))
		telemetry.InitAppInsights(applicationInsightsID, version)
		defer telemetry.ShutdownAppInsights()
		tel = telemetry.NewAppInsightsTelemetryClient("retina-capture", map[string]string{
			"version":                   version,
			telemetry.PropertyApiserver: os.Getenv(captureConstants.ApiserverEnvKey),
		})
	} else {
		tel = telemetry.NewNoopTelemetry()
	}

	// Create channel to listen for signals.
	sigChan := make(chan os.Signal, 1)
	// Notify sigChan for SIGTERM.
	signal.Notify(sigChan, syscall.SIGTERM)

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
