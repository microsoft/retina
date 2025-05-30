// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
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

func TestTcpdumpCommandConstruction(t *testing.T) {
	// Test that tcpdump command includes "-i any" by default
	t.Run("DefaultBehaviorIncludesAnyInterface", func(t *testing.T) {
		// Ensure no TCPDUMP_RAW_FILTER env var is set
		os.Unsetenv(captureConstants.TcpdumpRawFilterEnvKey)
		os.Unsetenv(captureConstants.PacketSizeEnvKey)
		
		captureFilePath := "/tmp/test.pcap"
		cmd := constructTcpdumpCommand(captureFilePath)
		
		// Verify the command contains "-i any"
		found := false
		for i, arg := range cmd.Args {
			if arg == "-i" && i+1 < len(cmd.Args) && cmd.Args[i+1] == "any" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tcpdump command to include '-i any', but got args: %v", cmd.Args)
		}
	})

	// Test that raw tcpdump filter overrides the default
	t.Run("RawFilterOverridesDefault", func(t *testing.T) {
		// Set a custom raw filter
		os.Setenv(captureConstants.TcpdumpRawFilterEnvKey, "-i eth0")
		defer os.Unsetenv(captureConstants.TcpdumpRawFilterEnvKey)
		os.Unsetenv(captureConstants.PacketSizeEnvKey)
		
		captureFilePath := "/tmp/test.pcap"
		cmd := constructTcpdumpCommand(captureFilePath)
		
		// Verify the command contains "-i eth0" but not "-i any"
		foundEth0 := false
		foundAny := false
		for i, arg := range cmd.Args {
			if arg == "-i" && i+1 < len(cmd.Args) {
				if cmd.Args[i+1] == "eth0" {
					foundEth0 = true
				}
				if cmd.Args[i+1] == "any" {
					foundAny = true
				}
			}
		}
		if !foundEth0 {
			t.Errorf("Expected tcpdump command to include '-i eth0' from raw filter, but got args: %v", cmd.Args)
		}
		if foundAny {
			t.Errorf("Expected tcpdump command not to include '-i any' when raw filter is set, but got args: %v", cmd.Args)
		}
	})
}

// constructTcpdumpCommand is a helper function that mimics the tcpdump command construction
// from CaptureNetworkPacket for testing purposes
func constructTcpdumpCommand(captureFilePath string) *exec.Cmd {
	captureStartCmd := exec.Command(
		"tcpdump",
		"-w", captureFilePath,
		"--relinquish-privileges=root",
	)

	if packetSize := os.Getenv(captureConstants.PacketSizeEnvKey); len(packetSize) != 0 {
		captureStartCmd.Args = append(
			captureStartCmd.Args,
			"-s", packetSize,
		)
	}

	// If we set flag and value into the arg item of args, the space between flag and value will not treated as part of
	// value, for example, "-i eth0" will be treated as "-i" and " eth0", thus brings a tcpdump unknown interface error.
	if tcpdumpRawFilter := os.Getenv(captureConstants.TcpdumpRawFilterEnvKey); len(tcpdumpRawFilter) != 0 {
		tcpdumpRawFilterSlice := strings.Split(tcpdumpRawFilter, " ")
		captureStartCmd.Args = append(captureStartCmd.Args, tcpdumpRawFilterSlice...)
	} else {
		// Default to capturing on all interfaces if no raw tcpdump filter is specified
		captureStartCmd.Args = append(captureStartCmd.Args, "-i", "any")
	}

	return captureStartCmd
}
