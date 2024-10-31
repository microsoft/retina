// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/microsoft/retina/pkg/log"
)

func TestSetupAndCleanup(t *testing.T) {
	captureName := "capture-test"
	nodeHostName := "node1"
	timestamp := file.Timestamp{Time: time.Now().UTC()}
	log.SetupZapLogger(log.GetDefaultLogOpts())
	networkCaptureprovider := &NetworkCaptureProvider{l: log.Logger().Named("test")}
	tmpFilename := file.CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: &timestamp}
	tmpCaptureLocation, err := networkCaptureprovider.Setup(tmpFilename)

	// remove temporary capture dir anyway in case Cleanup() fails.
	defer os.RemoveAll(tmpCaptureLocation)

	if err != nil {
		t.Errorf("Setup should have not fail with error %s", err)
	}
	if !strings.Contains(tmpCaptureLocation, captureName) {
		t.Errorf("Temporary capture dir name %s should contains capture name  %s", tmpCaptureLocation, captureName)
	}
	if !strings.Contains(tmpCaptureLocation, nodeHostName) {
		t.Errorf("Temporary capture dir name %s should contains node host name  %s", tmpCaptureLocation, nodeHostName)
	}
	if !strings.Contains(tmpCaptureLocation, timestamp.String()) {
		t.Errorf("Temporary capture dir name %s should contain timestamp  %s", tmpCaptureLocation, timestamp)
	}

	if _, err := os.Stat(tmpCaptureLocation); os.IsNotExist(err) {
		t.Errorf("Temporary capture dir %s should be created", tmpCaptureLocation)
	}

	err = networkCaptureprovider.Cleanup()
	if err != nil {
		t.Errorf("Cleanup should have not fail with error %s", err)
	}

	if _, err := os.Stat(tmpCaptureLocation); !os.IsNotExist(err) {
		t.Errorf("Temporary capture dir %s should be deleted", tmpCaptureLocation)
	}
}
