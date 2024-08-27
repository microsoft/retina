package main

import (
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/capture"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
)

func main() {
	lOpts := log.GetDefaultLogOpts()
	// Set Azure application insights ID if it is provided
	if buildinfo.ApplicationInsightsID != "" {
		lOpts.ApplicationInsightsID = buildinfo.ApplicationInsightsID
	}

	log.SetupZapLogger(lOpts)
	l := log.Logger().Named("captureworkload")
	l.Info("Start to capture network traffic")
	l.Info("Version: ", zap.String("version", buildinfo.Version))

	var tel telemetry.Telemetry
	var err error
	if buildinfo.ApplicationInsightsID != "" {
		l.Info("telemetry enabled", zap.String("applicationInsightsID", buildinfo.ApplicationInsightsID))
		telemetry.InitAppInsights(buildinfo.ApplicationInsightsID, buildinfo.Version)
		defer telemetry.ShutdownAppInsights()
		tel, err = telemetry.NewAppInsightsTelemetryClient("retina-capture", map[string]string{
			"version":                   buildinfo.Version,
			telemetry.PropertyApiserver: os.Getenv(captureConstants.ApiserverEnvKey),
		})
		if err != nil {
			log.Logger().Panic("failed to create telemetry client", zap.Error(err))
		}
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
