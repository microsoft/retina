//go:build windows

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/microsoft/retina/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateNetshFilter(t *testing.T) {
	tests := []struct {
		name      string
		filter    string
		expectErr bool
	}{
		{
			name:      "Valid IPv4 filter",
			filter:    "IPv4.Address=10.0.0.1",
			expectErr: false,
		},
		{
			name:      "Valid IPv4 filter with multiple addresses",
			filter:    "IPv4.Address=(10.244.1.85,10.244.1.235)",
			expectErr: false,
		},
		{
			name:      "Valid IPv6 filter",
			filter:    "IPv6.Address=(fd5c:d9f1:79c5:fd83::1bc,fd5c:d9f1:79c5:fd83::11b)",
			expectErr: false,
		},
		{
			name:      "Valid combined IPv4 and IPv6 filter",
			filter:    "IPv4.Address=(10.244.1.85,10.244.1.235) IPv6.Address=(fd5c:d9f1:79c5:fd83::1bc,fd5c:d9f1:79c5:fd83::11b)",
			expectErr: false,
		},
		{
			name:      "Shell injection with ampersand",
			filter:    "IPv4.Address=10.0.0.1 & powershell -enc <base64>",
			expectErr: true,
		},
		{
			name:      "Shell injection with pipe",
			filter:    "IPv4.Address=10.0.0.1 | powershell -Command <cmd>",
			expectErr: true,
		},
		{
			name:      "Shell injection with caret",
			filter:    "IPv4.Address=10.0.0.1 ^ powershell",
			expectErr: true,
		},
		{
			name:      "Shell injection with redirect",
			filter:    "IPv4.Address=10.0.0.1 > c:\\temp\\output.txt",
			expectErr: true,
		},
		{
			name:      "Shell injection with semicolon",
			filter:    "IPv4.Address=10.0.0.1; powershell",
			expectErr: true,
		},
		{
			name:      "Shell injection with dollar sign",
			filter:    "IPv4.Address=$env:TEMP",
			expectErr: true,
		},
		{
			name:      "Shell injection with backtick",
			filter:    "IPv4.Address=`powershell`",
			expectErr: true,
		},
		{
			name:      "Shell injection with double quotes",
			filter:    "IPv4.Address=\"10.0.0.1\"",
			expectErr: true,
		},
		{
			name:      "Shell injection with single quote",
			filter:    "IPv4.Address='10.0.0.1'",
			expectErr: true,
		},
		{
			name:      "Shell injection with percent",
			filter:    "IPv4.Address=%TEMP%",
			expectErr: true,
		},
		{
			name:      "Shell injection with backslash",
			filter:    "IPv4.Address=10.0.0.1\\powershell",
			expectErr: true,
		},
		{
			name:      "Shell injection with newline",
			filter:    "IPv4.Address=10.0.0.1\npowershell",
			expectErr: true,
		},
		{
			name:      "Empty filter",
			filter:    "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNetshFilter(tt.filter)
			if tt.expectErr && err == nil {
				t.Errorf("Expected error for filter '%s', but got none", tt.filter)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error for filter '%s', but got: %v", tt.filter, err)
			}
		})
	}
}

// TestStopNetworkCapture_ContextIndependence verifies stopNetworkCapture creates its own context
func TestStopNetworkCapture_ContextIndependence(t *testing.T) {
	now := metav1.Now()
	ncp := &NetworkCaptureProvider{
		NetworkCaptureProviderCommon: NetworkCaptureProviderCommon{
			TmpCaptureDir: t.TempDir(),
			l:             log.Logger().Named("test-capture"),
		},
		Filename: file.CaptureFilename{
			CaptureName:    "test-capture",
			NodeHostname:   "test-node",
			StartTimestamp: &now,
		},
	}

	// Create an expired context (simulating capture duration ending)
	parentCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	time.Sleep(10 * time.Millisecond)
	defer cancel()

	if parentCtx.Err() == nil {
		t.Fatal("Setup error: parent context should be expired")
	}

	// Call StopNetworkCapture - should NOT return "context deadline exceeded"
	err := ncp.stopNetworkCapture()

	if err != nil && err.Error() == "context deadline exceeded" {
		t.Fatal("StopNetworkCapture returned 'context deadline exceeded' - bug reintroduced")
	}

	t.Logf("StopNetworkCapture uses independent context (netsh error expected: %v)", err)
}

func TestCaptureNetworkPacketZeroDurationDoesNotCancelContext(t *testing.T) {
	// When duration=0 (e.g., rotating captures without a time limit),
	// CaptureNetworkPacket must NOT wrap the context with a zero timeout,
	// which would cancel immediately and prevent any capture from running.
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	now := metav1.Now()
	ncp := &NetworkCaptureProvider{
		NetworkCaptureProviderCommon: NetworkCaptureProviderCommon{
			l: log.Logger().Named("test-capture"),
		},
		TmpCaptureDir: t.TempDir(),
		Filename: file.CaptureFilename{
			CaptureName:    "test-zero-duration",
			NodeHostname:   "test-node",
			StartTimestamp: &now,
		},
		l: log.Logger().Named("test-capture"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call with duration=0 — this should NOT cancel the context immediately.
	// It will fail because netsh isn't available in the test environment,
	// but the error should NOT be "context deadline exceeded".
	err := ncp.CaptureNetworkPacket(ctx, "", 0, 100, 0)

	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatal("duration=0 caused immediate context cancellation — the zero-timeout bug is present")
	}

	// The context should still be valid (not expired) after the call
	if ctx.Err() != nil {
		t.Fatalf("Parent context should not be cancelled, but got: %v", ctx.Err())
	}

	t.Logf("duration=0 correctly skips context timeout wrapping (netsh error expected: %v)", err)
}
