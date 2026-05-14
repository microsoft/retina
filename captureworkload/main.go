package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
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

	// Create a context that is canceled when a termination signal is received
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()

	cm := capture.NewCaptureManager(l, tel)

	defer func() {
		if err := cm.Cleanup(); err != nil {
			l.Error("Failed to cleanup network capture", zap.Error(err))
		}
	}()
	srcDir, err := cm.CaptureNetwork(ctx)
	if err != nil {
		l.Error("Failed to capture network traffic", zap.Error(err))
		os.Exit(1)
	}
	if err := handleOutputResult(cm.OutputCapture(ctx, srcDir), l); err != nil {
		l.Error("Failed to output network traffic", zap.Error(err))
		os.Exit(1)
	}
	l.Info("Done for capturing network traffic")
}

// handleOutputResult returns the output error if non-nil (signaling the caller
// should exit non-zero), otherwise runs host-path cleanup and returns nil.
func handleOutputResult(outputErr error, l *log.ZapLogger) error {
	if outputErr != nil {
		return outputErr
	}
	cleanupHostPathCaptureFiles(l)
	return nil
}

// cleanupHostPathCaptureFiles removes capture files from the node host path
// after a successful upload to a remote output location (blob, S3, or PVC).
// It is a no-op when:
// - CLEANUP_HOST_PATH env is not "true"
// - host path is the only configured output (refuses to destroy only copy)
// - no remote output is configured
func cleanupHostPathCaptureFiles(l *log.ZapLogger) {
	cleanupStr := os.Getenv(captureConstants.CleanupHostPathEnvKey)
	if cleanupStr == "" {
		return
	}
	cleanup, err := strconv.ParseBool(cleanupStr)
	if err != nil || !cleanup {
		return
	}

	hostPath := os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyHostPath))
	if hostPath == "" {
		// Host path not configured, nothing to clean up.
		return
	}

	// Check that at least one remote output location is configured.
	hasRemoteOutput := os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim)) != "" ||
		os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Bucket)) != ""
	if !hasRemoteOutput {
		// Check for blob upload secret mount as indicator of blob output.
		if _, err := os.Stat(captureConstants.CaptureOutputLocationBlobUploadSecretPath); os.IsNotExist(err) {
			l.Info("Skipping host-path cleanup: no remote output configured, host path is the only copy")
			return
		}
	}

	// Remove capture files from host path directory.
	entries, err := os.ReadDir(hostPath)
	if err != nil {
		l.Error("Failed to read host path directory for cleanup", zap.String("hostPath", hostPath), zap.Error(err))
		return
	}

	captureName := os.Getenv(captureConstants.CaptureNameEnvKey)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Only remove files that match the capture name prefix to avoid deleting unrelated files.
		if captureName != "" && !matchesCaptureFile(entry.Name(), captureName) {
			continue
		}
		filePath := filepath.Join(hostPath, entry.Name())
		if err := os.Remove(filePath); err != nil {
			l.Error("Failed to remove capture file from host path", zap.String("file", filePath), zap.Error(err))
		} else {
			l.Info("Cleaned up capture file from host path", zap.String("file", filePath))
		}
	}
}

// matchesCaptureFile checks whether a filename belongs to this capture based on name prefix.
func matchesCaptureFile(filename, captureName string) bool {
	return len(filename) >= len(captureName) && filename[:len(captureName)] == captureName
}
