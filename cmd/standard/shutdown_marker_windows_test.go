//go:build windows

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchShutdownMarkerCancelsContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	markerPath := filepath.Join(t.TempDir(), shutdownMarkerName)
	watcherDone := make(chan struct{})

	go func() {
		defer close(watcherDone)
		watchShutdownMarker(ctx, cancel, markerPath, time.Millisecond)
	}()

	if err := os.WriteFile(markerPath, nil, 0o600); err != nil {
		t.Fatalf("creating shutdown marker: %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("context was not cancelled after creating shutdown marker")
	}

	select {
	case <-watcherDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown marker watcher did not stop after cancelling context")
	}
}

func TestWatchShutdownMarkerStopsWithContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	markerPath := filepath.Join(t.TempDir(), shutdownMarkerName)
	watcherDone := make(chan struct{})

	go func() {
		defer close(watcherDone)
		watchShutdownMarker(ctx, cancel, markerPath, time.Millisecond)
	}()

	cancel()

	select {
	case <-watcherDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown marker watcher did not stop after context cancellation")
	}
}
