//go:build unix

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/microsoft/retina/pkg/log"
)

const (
	testCaptureFilePath = "/tmp/test.pcap"
	interfaceEth0       = "eth0"
	interfaceEth1       = "eth1"
	interfaceAny        = "any"
	interfaceLo         = "lo"
)

func TestSetupAndCleanup(t *testing.T) {
	captureName := "capture-test"
	nodeHostName := "node1"
	timestamp := file.Now()
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	networkCaptureprovider := NewNetworkCaptureProvider(log.Logger().Named("test"))
	tmpFilename := file.CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: timestamp}
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
	if !strings.Contains(tmpCaptureLocation, file.TimeToString(timestamp)) {
		t.Errorf("Temporary capture dir name %s should contain timestamp  %s", tmpCaptureLocation, timestamp)
	}

	if _, statErr := os.Stat(tmpCaptureLocation); os.IsNotExist(statErr) {
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

// Helper function to check if command args contain specific interface
func hasInterface(cmd *exec.Cmd, expectedInterface string) bool {
	for i, arg := range cmd.Args {
		if arg == "-i" && i+1 < len(cmd.Args) && cmd.Args[i+1] == expectedInterface {
			return true
		}
	}
	return false
}

// Helper function to reset environment variables
func resetEnvVars() {
	os.Unsetenv(captureConstants.TcpdumpRawFilterEnvKey)
	os.Unsetenv(captureConstants.PacketSizeEnvKey)
	os.Unsetenv(captureConstants.CaptureInterfacesEnvKey)
}

func TestTcpdumpDefaultBehavior(t *testing.T) {
	resetEnvVars()

	cmd := constructTcpdumpCommand(testCaptureFilePath)

	if !hasInterface(cmd, interfaceAny) {
		t.Errorf("Expected tcpdump command to include '-i any', but got args: %v", cmd.Args)
	}
}

func TestTcpdumpRawFilterOverride(t *testing.T) {
	resetEnvVars()
	os.Setenv(captureConstants.TcpdumpRawFilterEnvKey, "-i "+interfaceEth0)
	defer os.Unsetenv(captureConstants.TcpdumpRawFilterEnvKey)

	cmd := constructTcpdumpCommand(testCaptureFilePath)

	if !hasInterface(cmd, interfaceEth0) {
		t.Errorf("Expected tcpdump command to include '-i %s' from raw filter, but got args: %v", interfaceEth0, cmd.Args)
	}
	if hasInterface(cmd, interfaceAny) {
		t.Errorf("Expected tcpdump command not to include '-i any' when raw filter is set, but got args: %v", cmd.Args)
	}
}

func TestTcpdumpSpecificInterfaces(t *testing.T) {
	resetEnvVars()
	os.Setenv(captureConstants.CaptureInterfacesEnvKey, interfaceEth0+","+interfaceEth1)
	defer os.Unsetenv(captureConstants.CaptureInterfacesEnvKey)

	cmd := constructTcpdumpCommand(testCaptureFilePath)

	if !hasInterface(cmd, interfaceEth0) {
		t.Errorf("Expected tcpdump command to include '-i %s', but got args: %v", interfaceEth0, cmd.Args)
	}
	if !hasInterface(cmd, interfaceEth1) {
		t.Errorf("Expected tcpdump command to include '-i %s', but got args: %v", interfaceEth1, cmd.Args)
	}
	if hasInterface(cmd, interfaceAny) {
		t.Errorf("Expected tcpdump command not to include '-i any' when specific interfaces are set, but got args: %v", cmd.Args)
	}
}

func TestTcpdumpRawFilterPriority(t *testing.T) {
	resetEnvVars()
	os.Setenv(captureConstants.TcpdumpRawFilterEnvKey, "-i "+interfaceLo)
	os.Setenv(captureConstants.CaptureInterfacesEnvKey, interfaceEth0+","+interfaceEth1)
	defer os.Unsetenv(captureConstants.TcpdumpRawFilterEnvKey)
	defer os.Unsetenv(captureConstants.CaptureInterfacesEnvKey)

	cmd := constructTcpdumpCommand(testCaptureFilePath)

	if !hasInterface(cmd, interfaceLo) {
		t.Errorf("Expected tcpdump command to include '-i %s' from raw filter, but got args: %v", interfaceLo, cmd.Args)
	}
	if hasInterface(cmd, interfaceEth0) || hasInterface(cmd, interfaceEth1) {
		t.Errorf("Expected tcpdump command not to include specific interfaces when raw filter is set, but got args: %v", cmd.Args)
	}
}

func TestTcpdumpInterfaceOverrideDefault(t *testing.T) {
	resetEnvVars()
	os.Setenv(captureConstants.CaptureInterfacesEnvKey, interfaceEth0)
	defer os.Unsetenv(captureConstants.CaptureInterfacesEnvKey)

	cmd := constructTcpdumpCommand(testCaptureFilePath)

	if !hasInterface(cmd, interfaceEth0) {
		t.Errorf("Expected tcpdump command to include '-i %s' from specific interfaces, but got args: %v", interfaceEth0, cmd.Args)
	}
	if hasInterface(cmd, interfaceAny) {
		t.Errorf("Expected tcpdump command not to include '-i any' when specific interfaces are set, but got args: %v", cmd.Args)
	}
}

func TestTcpdumpCommandConstruction(t *testing.T) {
	t.Run("DefaultBehaviorIncludesAnyInterface", TestTcpdumpDefaultBehavior)
	t.Run("RawFilterOverridesDefault", TestTcpdumpRawFilterOverride)
	t.Run("SpecificInterfaceSelection", TestTcpdumpSpecificInterfaces)
	t.Run("RawFilterOverridesSpecificInterfaces", TestTcpdumpRawFilterPriority)
	t.Run("SpecificInterfacesOverrideDefault", TestTcpdumpInterfaceOverrideDefault)
}
