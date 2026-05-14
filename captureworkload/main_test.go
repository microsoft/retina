// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

func TestCleanupHostPathCaptureFiles(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test")

	t.Run("no-op when env not set", func(t *testing.T) {
		t.Setenv(captureConstants.CleanupHostPathEnvKey, "")
		cleanupHostPathCaptureFiles(l)
	})

	t.Run("no-op when cleanup is false", func(t *testing.T) {
		t.Setenv(captureConstants.CleanupHostPathEnvKey, "false")
		cleanupHostPathCaptureFiles(l)
	})

	t.Run("no-op when host path not configured", func(t *testing.T) {
		t.Setenv(captureConstants.CleanupHostPathEnvKey, "true")
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyHostPath), "")
		cleanupHostPathCaptureFiles(l)
	})

	t.Run("no-op when host path is only output", func(t *testing.T) {
		hostDir := t.TempDir()
		// Create a capture file
		if err := os.WriteFile(filepath.Join(hostDir, "my-capture-node1.tar.gz"), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}

		t.Setenv(captureConstants.CleanupHostPathEnvKey, "true")
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyHostPath), hostDir)
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim), "")
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Bucket), "")
		t.Setenv(captureConstants.CaptureNameEnvKey, "my-capture")

		cleanupHostPathCaptureFiles(l)

		// File should still exist since host path is the only output.
		if _, err := os.Stat(filepath.Join(hostDir, "my-capture-node1.tar.gz")); os.IsNotExist(err) {
			t.Error("capture file was deleted when host path was the only output")
		}
	})

	t.Run("cleans up when remote output is configured via S3", func(t *testing.T) {
		hostDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(hostDir, "my-capture-node1.tar.gz"), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
		// Also create an unrelated file that should NOT be deleted
		if err := os.WriteFile(filepath.Join(hostDir, "other-file.txt"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}

		t.Setenv(captureConstants.CleanupHostPathEnvKey, "true")
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyHostPath), hostDir)
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Bucket), "my-bucket")
		t.Setenv(captureConstants.CaptureNameEnvKey, "my-capture")

		cleanupHostPathCaptureFiles(l)

		// Capture file should be removed.
		if _, err := os.Stat(filepath.Join(hostDir, "my-capture-node1.tar.gz")); !os.IsNotExist(err) {
			t.Error("capture file was not deleted after remote upload")
		}
		// Unrelated file should remain.
		if _, err := os.Stat(filepath.Join(hostDir, "other-file.txt")); os.IsNotExist(err) {
			t.Error("unrelated file was incorrectly deleted")
		}
	})

	t.Run("cleans up when remote output is configured via PVC", func(t *testing.T) {
		hostDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(hostDir, "test-cap-host1.tar.gz"), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}

		t.Setenv(captureConstants.CleanupHostPathEnvKey, "true")
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyHostPath), hostDir)
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim), "my-pvc")
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Bucket), "")
		t.Setenv(captureConstants.CaptureNameEnvKey, "test-cap")

		cleanupHostPathCaptureFiles(l)

		if _, err := os.Stat(filepath.Join(hostDir, "test-cap-host1.tar.gz")); !os.IsNotExist(err) {
			t.Error("capture file was not deleted after PVC upload")
		}
	})
}

func TestHandleOutputResult(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test")

	t.Run("returns error when output fails", func(t *testing.T) {
		outputErr := errors.New("upload failed: 403 AuthorizationPermissionMismatch")
		err := handleOutputResult(outputErr, l)
		if err == nil {
			t.Fatal("expected error to be returned when output fails")
		}
		if err != outputErr {
			t.Errorf("expected original error, got %v", err)
		}
	})

	t.Run("returns nil and runs cleanup on success", func(t *testing.T) {
		hostDir := t.TempDir()
		captureFile := filepath.Join(hostDir, "my-capture-node1.tar.gz")
		if err := os.WriteFile(captureFile, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}

		t.Setenv(captureConstants.CleanupHostPathEnvKey, "true")
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyHostPath), hostDir)
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Bucket), "my-bucket")
		t.Setenv(captureConstants.CaptureNameEnvKey, "my-capture")

		err := handleOutputResult(nil, l)
		if err != nil {
			t.Fatalf("expected nil error on success, got %v", err)
		}

		// Capture file should have been cleaned up.
		if _, err := os.Stat(captureFile); !os.IsNotExist(err) {
			t.Error("capture file should have been deleted after successful output")
		}
	})

	t.Run("does not run cleanup when output fails", func(t *testing.T) {
		hostDir := t.TempDir()
		captureFile := filepath.Join(hostDir, "my-capture-node1.tar.gz")
		if err := os.WriteFile(captureFile, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}

		t.Setenv(captureConstants.CleanupHostPathEnvKey, "true")
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyHostPath), hostDir)
		t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Bucket), "my-bucket")
		t.Setenv(captureConstants.CaptureNameEnvKey, "my-capture")

		_ = handleOutputResult(errors.New("upload failed"), l)

		// Capture file should NOT have been cleaned up.
		if _, err := os.Stat(captureFile); os.IsNotExist(err) {
			t.Error("capture file should NOT be deleted when output fails")
		}
	})
}

func TestMatchesCaptureFile(t *testing.T) {
	tests := []struct {
		filename    string
		captureName string
		want        bool
	}{
		{"my-capture-node1.tar.gz", "my-capture", true},
		{"my-capture", "my-capture", true},
		{"other-file.txt", "my-capture", false},
		{"", "my-capture", false},
		{"my-cap", "my-capture", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := matchesCaptureFile(tt.filename, tt.captureName)
			if got != tt.want {
				t.Errorf("matchesCaptureFile(%q, %q) = %v, want %v", tt.filename, tt.captureName, got, tt.want)
			}
		})
	}
}
