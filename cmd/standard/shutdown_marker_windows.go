//go:build windows

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standard

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	shutdownMarkerName         = ".retina-shutdown"
	shutdownMarkerPollInterval = 5 * time.Second
)

func setupShutdownMarker(ctx context.Context, cancel context.CancelFunc) {
	executablePath, err := os.Executable()
	if err != nil {
		slog.Error("failed to resolve controller executable", "error", err)
		return
	}

	go watchShutdownMarker(
		ctx,
		cancel,
		filepath.Join(filepath.Dir(executablePath), shutdownMarkerName),
		shutdownMarkerPollInterval,
	)
}

func watchShutdownMarker(ctx context.Context, cancel context.CancelFunc, path string, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := os.Stat(path)
			switch {
			case err == nil:
				slog.Info("shutdown marker detected", "path", path)
				cancel()
				return
			case errors.Is(err, os.ErrNotExist):
				continue
			default:
				slog.Error("failed to check shutdown marker", "path", path, "error", err)
			}
		}
	}
}
