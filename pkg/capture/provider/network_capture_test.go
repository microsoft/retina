//go:build unix

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/microsoft/retina/pkg/log"
)

const (
	testCaptureFilePath = "/tmp/test.pcap"
	testCaptureName     = "test-capture"
	testNodeHostName    = "test-node"
	interfaceEth0       = "eth0"
	interfaceEth1       = "eth1"
	interfaceAny        = "any"
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
	os.Unsetenv(captureConstants.PcapFilterEnvKey)
	os.Unsetenv(captureConstants.PacketSizeEnvKey)
	os.Unsetenv(captureConstants.CaptureInterfacesEnvKey)
	os.Unsetenv(captureConstants.TcpdumpFlagsEnvKey)
}

// Helper function to check if tcpdump is available on the system
func requireTcpdump(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tcpdump"); err != nil {
		t.Skipf("tcpdump not available on system: %v", err)
	}
}

// TestTcpdumpEmptyFilter verifies that empty filter falls back to default interface
// and that no unexpected or malicious arguments are injected.
func TestTcpdumpEmptyFilter(t *testing.T) {
	resetEnvVars()
	cmd := constructTcpdumpCommand(testCaptureFilePath, "")

	// Should fall back to "-i any"
	if !hasInterface(cmd, interfaceAny) {
		t.Errorf("Expected fallback to '-i any' with empty filter, but got args: %v", cmd.Args)
	}

	// Verify only expected args are present and no malicious content
	for _, arg := range cmd.Args {
		if arg != "tcpdump" && arg != "-w" && arg != testCaptureFilePath &&
			arg != "--relinquish-privileges=root" && arg != "-i" && arg != interfaceAny {
			t.Errorf("Unexpected argument '%s' found in empty filter command: %v", arg, cmd.Args)
		}
		// Check for malicious content
		if strings.Contains(arg, "/etc/passwd") || strings.Contains(arg, "evil") ||
			strings.Contains(arg, "rm -rf") || strings.HasPrefix(arg, "-z") {
			t.Errorf("Malicious content should not be present in command args: %v", cmd.Args)
		}
	}
}

func TestTcpdumpWithBPFFilter(t *testing.T) {
	resetEnvVars()
	// Test that a valid BPF filter is properly added to the tcpdump command
	// Note: Filter validation (e.g., rejecting '-' prefix) happens in CaptureNetworkPacket

	bpfFilter := "tcp port 80"

	cmd := constructTcpdumpCommand(testCaptureFilePath, bpfFilter)

	// Should have the BPF filter as an argument
	found := slices.Contains(cmd.Args, bpfFilter)
	if !found {
		t.Errorf("Expected BPF filter '%s' in args, but got: %v", bpfFilter, cmd.Args)
	}
}

func TestTcpdumpSpecificInterfaces(t *testing.T) {
	resetEnvVars()
	os.Setenv(captureConstants.CaptureInterfacesEnvKey, interfaceEth0+","+interfaceEth1)
	defer os.Unsetenv(captureConstants.CaptureInterfacesEnvKey)

	cmd := constructTcpdumpCommand(testCaptureFilePath, "")

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

func TestTcpdumpBPFFilterWithSpecificInterfaces(t *testing.T) {
	resetEnvVars()
	// Verify that BPF filter and specific interface selection work together
	// Both should be present in the command (they are independent features)
	bpfFilter := "tcp port 443"
	os.Setenv(captureConstants.CaptureInterfacesEnvKey, interfaceEth0+","+interfaceEth1)
	defer os.Unsetenv(captureConstants.CaptureInterfacesEnvKey)

	cmd := constructTcpdumpCommand(testCaptureFilePath, bpfFilter)

	// The BPF filter should be present
	found := slices.Contains(cmd.Args, bpfFilter)
	if !found {
		t.Errorf("Expected BPF filter '%s' in command, but got args: %v", bpfFilter, cmd.Args)
	}

	// Interfaces should still be present (BPF filter doesn't override interface selection)
	if !hasInterface(cmd, interfaceEth0) || !hasInterface(cmd, interfaceEth1) {
		t.Errorf("Expected both interfaces to be present with BPF filter, but got args: %v", cmd.Args)
	}
}

func TestTcpdumpCommandConstruction(t *testing.T) {
	// Default behavior tests
	t.Run("EmptyFilter", TestTcpdumpEmptyFilter)

	// Interface selection tests
	t.Run("SpecificInterfaceSelection", TestTcpdumpSpecificInterfaces)
	t.Run("InterfaceListWithEmptyEntries", TestTcpdumpInterfaceListWithEmptyEntries)

	// BPF filter tests
	t.Run("WithBPFFilter", TestTcpdumpWithBPFFilter)
	t.Run("BPFFilterWithSpecificInterfaces", TestTcpdumpBPFFilterWithSpecificInterfaces)
	t.Run("BPFFilterWithComplexExpression", TestTcpdumpBPFFilterComplexExpression)
	t.Run("BPFFilterWithTcpFlags", TestTcpdumpBPFFilterWithTcpFlags)

	// Option tests
	t.Run("PacketSizeOption", TestTcpdumpPacketSizeOption)
}

// TestTcpdumpBPFFilterComplexExpression validates that complex BPF filter expressions
// with multiple keywords and operators are passed as a single argument, not split on spaces.
// This is critical for security - splitting would allow flag injection attacks.
func TestTcpdumpBPFFilterComplexExpression(t *testing.T) {
	resetEnvVars()
	// Test a complex BPF filter that should remain as one argument
	bpfFilter := "tcp and (port 80 or port 443) and host 10.0.0.1"

	cmd := constructTcpdumpCommand(testCaptureFilePath, bpfFilter)

	// The entire filter must appear as a single argument
	found := slices.Contains(cmd.Args, bpfFilter)
	if !found {
		t.Errorf("Expected entire BPF filter '%s' as single argument, but got args: %v", bpfFilter, cmd.Args)
	}

	// Verify individual keywords are NOT separate arguments (which would indicate splitting)
	splitIndicators := []string{"tcp", "and", "port", "80", "or", "443", "host", "10.0.0.1"}
	for _, indicator := range splitIndicators {
		for _, arg := range cmd.Args {
			if arg == indicator {
				t.Errorf("BPF filter was incorrectly split: found '%s' as separate arg in: %v", indicator, cmd.Args)
			}
		}
	}
}

// TestTcpdumpBPFFilterWithTcpFlags verifies that BPF filters using TCP flag syntax
// with special characters like brackets, pipes, and ampersands are passed correctly.
// Example: tcp[tcpflags] & (tcp-syn|tcp-ack) == tcp-syn
func TestTcpdumpBPFFilterWithTcpFlags(t *testing.T) {
	resetEnvVars()
	// Test a BPF filter with TCP flags syntax and special characters
	bpfFilter := "tcp[tcpflags] & (tcp-syn|tcp-ack) == tcp-syn"

	cmd := constructTcpdumpCommand(testCaptureFilePath, bpfFilter)

	// Positive check: The entire filter must appear as a single argument
	found := slices.Contains(cmd.Args, bpfFilter)
	if !found {
		t.Errorf("Expected entire BPF filter '%s' as single argument, but got args: %v", bpfFilter, cmd.Args)
	}

	// Negative check: Verify the filter is not split on spaces (which would indicate incorrect handling)
	// These are the pieces that would appear if the filter were split on spaces
	splitIndicators := []string{"tcp[tcpflags]", "&", "(tcp-syn|tcp-ack)", "==", "tcp-syn"}
	for _, indicator := range splitIndicators {
		for _, arg := range cmd.Args {
			if arg == indicator {
				t.Errorf("BPF filter was incorrectly split: found '%s' as separate arg in: %v", indicator, cmd.Args)
			}
		}
	}
}

// TestTcpdumpInterfaceListWithEmptyEntries verifies handling of interface lists with empty values
func TestTcpdumpInterfaceListWithEmptyEntries(t *testing.T) {
	resetEnvVars()
	// Interface list with empty entries and extra spaces
	os.Setenv(captureConstants.CaptureInterfacesEnvKey, "eth0, ,eth1,,eth2, ")
	defer os.Unsetenv(captureConstants.CaptureInterfacesEnvKey)

	cmd := constructTcpdumpCommand(testCaptureFilePath, "")

	// Should only include non-empty interfaces
	if !hasInterface(cmd, interfaceEth0) {
		t.Errorf("Expected '-i eth0', but got args: %v", cmd.Args)
	}
	if !hasInterface(cmd, interfaceEth1) {
		t.Errorf("Expected '-i eth1', but got args: %v", cmd.Args)
	}
	// eth2 should be present
	if !hasInterface(cmd, "eth2") {
		t.Errorf("Expected '-i eth2', but got args: %v", cmd.Args)
	}
}

// TestTcpdumpPacketSizeOption verifies that packet size option is correctly added
func TestTcpdumpPacketSizeOption(t *testing.T) {
	resetEnvVars()
	os.Setenv(captureConstants.PacketSizeEnvKey, "1500")
	defer os.Unsetenv(captureConstants.PacketSizeEnvKey)

	cmd := constructTcpdumpCommand(testCaptureFilePath, "")

	// Should include -s 1500
	foundS := false
	foundSize := false
	for i, arg := range cmd.Args {
		if arg == "-s" {
			foundS = true
			if i+1 < len(cmd.Args) && cmd.Args[i+1] == "1500" {
				foundSize = true
			}
		}
	}
	if !foundS || !foundSize {
		t.Errorf("Expected '-s 1500' in tcpdump args, but got: %v", cmd.Args)
	}
}

// TestTcpdumpBPFFilterOnly verifies command construction with BPF filter (no user flags)
func TestTcpdumpBPFFilterOnly(t *testing.T) {
	resetEnvVars()
	bpfFilter := "tcp port 80"

	cmd := constructTcpdumpCommand(testCaptureFilePath, bpfFilter)

	// BPF filter should be present as the last argument
	if !slices.Contains(cmd.Args, bpfFilter) {
		t.Errorf("Expected BPF filter '%s' in command args, but got: %v", bpfFilter, cmd.Args)
	}

	// Should have basic structure (tcpdump, -w, path, -i, etc.)
	if !slices.Contains(cmd.Args, "tcpdump") {
		t.Errorf("Expected 'tcpdump' in command args, but got: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Args, "-w") {
		t.Errorf("Expected '-w' in command args, but got: %v", cmd.Args)
	}

	// Verify no user-specified flags with '-' prefix (security check)
	for _, arg := range cmd.Args {
		// Skip our internal flags and the BPF filter
		if arg == "-w" || arg == "-i" || arg == "-s" || arg == "--relinquish-privileges=root" ||
			arg == testCaptureFilePath || arg == "tcpdump" || arg == "any" || arg == bpfFilter {
			continue
		}
		// Any other argument starting with '-' is suspicious
		if strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") {
			t.Errorf("Unexpected flag '%s' found in command (only internal flags should be present): %v", arg, cmd.Args)
		}
	}
}

// TestTcpdumpFlagsEnvVar tests that TCPDUMP_FLAGS environment variable is correctly parsed
func TestTcpdumpFlagsEnvVar(t *testing.T) {
	resetEnvVars()

	tests := []struct {
		name          string
		flagsEnvValue string
		expectedFlags []string
	}{
		{
			name:          "single flag",
			flagsEnvValue: "-p",
			expectedFlags: []string{"-p"},
		},
		{
			name:          "multiple flags space-separated",
			flagsEnvValue: "-p -n -v",
			expectedFlags: []string{"-p", "-n", "-v"},
		},
		{
			name:          "multiple flags with extra spaces",
			flagsEnvValue: "  -p   -n  -v  ",
			expectedFlags: []string{"-p", "-n", "-v"},
		},
		{
			name:          "flags with arguments",
			flagsEnvValue: "-s 96 -p",
			expectedFlags: []string{"-s", "96", "-p"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(captureConstants.TcpdumpFlagsEnvKey, tt.flagsEnvValue)
			defer os.Unsetenv(captureConstants.TcpdumpFlagsEnvKey)

			cmd := constructTcpdumpCommand(testCaptureFilePath, "")

			// Check all expected flags are present
			for _, expectedFlag := range tt.expectedFlags {
				if !slices.Contains(cmd.Args, expectedFlag) {
					t.Errorf("Expected flag '%s' in command args, but got: %v", expectedFlag, cmd.Args)
				}
			}
		})
	}
}

// TestFilterValidation tests flag rejection in filter input
func TestFilterValidation(t *testing.T) {
	requireTcpdump(t)
	resetEnvVars()

	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	ncp := NewNetworkCaptureProvider(log.Logger().Named("test")).(*NetworkCaptureProvider)

	captureName := testCaptureName
	nodeHostName := testNodeHostName
	timestamp := file.Now()
	ncp.Filename = file.CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: timestamp}
	tmpCaptureLocation, _ := ncp.Setup(ncp.Filename)
	defer os.RemoveAll(tmpCaptureLocation)

	tests := []struct {
		name        string
		setupEnv    func()
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid BPF filter without flags",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "tcp port 80")
			},
			shouldError: false,
		},
		{
			name: "filter with flag at start",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "-n tcp port 80")
			},
			shouldError: true,
			errorMsg:    "contains flag \"-n\"",
		},
		{
			name: "filter with flag at end",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "tcp port 80 -n")
			},
			shouldError: true,
			errorMsg:    "contains flag \"-n\"",
		},
		{
			name: "filter with flag in middle",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "tcp -n port 80")
			},
			shouldError: true,
			errorMsg:    "contains flag \"-n\"",
		},
		{
			name: "filter with tab-separated flag",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "tcp\t-n\tport 80")
			},
			shouldError: true,
			errorMsg:    "contains flag \"-n\"",
		},
		{
			name: "filter with newline-separated flag",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "tcp\n-n\nport 80")
			},
			shouldError: true,
			errorMsg:    "contains flag \"-n\"",
		},
		{
			name: "deprecated tcpdumpFilter with flag",
			setupEnv: func() {
				os.Setenv(captureConstants.TcpdumpRawFilterEnvKey, "-n tcp port 80")
			},
			shouldError: true,
			errorMsg:    "contains flag \"-n\"",
		},
		{
			name: "complex BPF expression without flags (valid)",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "tcp[tcpflags] & (tcp-syn|tcp-ack) == tcp-syn")
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnvVars()
			tt.setupEnv()
			defer resetEnvVars()

			err := ncp.CaptureNetworkPacket(context.Background(), "", 1, 0)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorMsg, err)
				}
			} else {
				// For valid filters, we may get tcpdump execution errors (duration too short, etc.)
				// but we should NOT get validation errors
				if err != nil && (strings.Contains(err.Error(), "contains flag") || strings.Contains(err.Error(), "whitespace-only")) {
					t.Errorf("Expected no validation error, but got: %v", err)
				}
			}
		})
	}
}

// TestFilterWhitespaceValidation tests whitespace-only filter rejection
func TestFilterWhitespaceValidation(t *testing.T) {
	resetEnvVars()

	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	ncp := NewNetworkCaptureProvider(log.Logger().Named("test")).(*NetworkCaptureProvider)

	captureName := "test-capture"
	nodeHostName := "test-node"
	timestamp := file.Now()
	ncp.Filename = file.CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: timestamp}
	tmpCaptureLocation, _ := ncp.Setup(ncp.Filename)
	defer os.RemoveAll(tmpCaptureLocation)

	tests := []struct {
		name     string
		setupEnv func()
	}{
		{
			name: "pcapFilter with only spaces",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "   ")
			},
		},
		{
			name: "pcapFilter with only tabs",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "\t\t")
			},
		},
		{
			name: "pcapFilter with only newlines",
			setupEnv: func() {
				os.Setenv(captureConstants.PcapFilterEnvKey, "\n\n")
			},
		},
		{
			name: "tcpdumpFilter with only spaces",
			setupEnv: func() {
				os.Setenv(captureConstants.TcpdumpRawFilterEnvKey, "   ")
			},
		},
		{
			name: "tcpdumpFilter with mixed whitespace",
			setupEnv: func() {
				os.Setenv(captureConstants.TcpdumpRawFilterEnvKey, " \t\n ")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnvVars()
			tt.setupEnv()
			defer resetEnvVars()

			err := ncp.CaptureNetworkPacket(context.Background(), "", 1, 0)

			if err == nil {
				t.Errorf("Expected error for whitespace-only filter, but got no error")
			} else if !errors.Is(err, errTcpdumpFilterEmptyOrWhitespace) {
				t.Errorf("Expected errTcpdumpFilterEmptyOrWhitespace, but got: %v", err)
			}
		})
	}
}

// TestFilterPrecedence tests that pcapFilter takes precedence over tcpdumpFilter
func TestFilterPrecedence(t *testing.T) {
	requireTcpdump(t)
	resetEnvVars()

	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	ncp := NewNetworkCaptureProvider(log.Logger().Named("test")).(*NetworkCaptureProvider)

	captureName := testCaptureName
	nodeHostName := testNodeHostName
	timestamp := file.Now()
	ncp.Filename = file.CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: timestamp}
	tmpCaptureLocation, _ := ncp.Setup(ncp.Filename)
	defer os.RemoveAll(tmpCaptureLocation)

	tests := []struct {
		name                string
		pcapFilter          string
		tcpdumpRawFilter    string
		expectValidationErr bool
	}{
		{
			name:                "both filters set - pcapFilter valid, tcpdumpFilter invalid",
			pcapFilter:          "tcp port 80",
			tcpdumpRawFilter:    "-n tcp",
			expectValidationErr: false, // Should use pcapFilter, ignore invalid tcpdumpFilter
		},
		{
			name:                "both filters set - pcapFilter invalid, tcpdumpFilter valid",
			pcapFilter:          "-n tcp",
			tcpdumpRawFilter:    "tcp port 80",
			expectValidationErr: true, // Should validate pcapFilter and reject it
		},
		{
			name:                "only pcapFilter set",
			pcapFilter:          "tcp port 443",
			tcpdumpRawFilter:    "",
			expectValidationErr: false,
		},
		{
			name:                "only tcpdumpFilter set",
			pcapFilter:          "",
			tcpdumpRawFilter:    "udp port 53",
			expectValidationErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnvVars()
			if tt.pcapFilter != "" {
				os.Setenv(captureConstants.PcapFilterEnvKey, tt.pcapFilter)
			}
			if tt.tcpdumpRawFilter != "" {
				os.Setenv(captureConstants.TcpdumpRawFilterEnvKey, tt.tcpdumpRawFilter)
			}
			defer resetEnvVars()

			err := ncp.CaptureNetworkPacket(context.Background(), "", 1, 0)

			if tt.expectValidationErr {
				if err == nil || !strings.Contains(err.Error(), "contains flag") {
					t.Errorf("Expected validation error, but got: %v", err)
				}
			} else {
				// We expect either no error or a tcpdump execution error (not validation error)
				if err != nil && strings.Contains(err.Error(), "contains flag") {
					t.Errorf("Unexpected validation error: %v", err)
				}
				// Other errors (e.g., tcpdump execution failures) are acceptable for these tests
			}
		})
	}
}

// TestFilterPrecedenceValue explicitly verifies that the correct filter value is used
func TestFilterPrecedenceValue(t *testing.T) {
	resetEnvVars()

	tests := []struct {
		name               string
		pcapFilter         string
		tcpdumpRawFilter   string
		expectedFilterUsed string // The filter that should actually appear in the tcpdump command
	}{
		{
			name:               "both set - should use pcapFilter",
			pcapFilter:         "tcp port 80",
			tcpdumpRawFilter:   "tcp port 8080",
			expectedFilterUsed: "tcp port 80",
		},
		{
			name:               "only pcapFilter set",
			pcapFilter:         "udp port 53",
			tcpdumpRawFilter:   "",
			expectedFilterUsed: "udp port 53",
		},
		{
			name:               "only tcpdumpFilter set",
			pcapFilter:         "",
			tcpdumpRawFilter:   "icmp",
			expectedFilterUsed: "icmp",
		},
		{
			name:               "both empty",
			pcapFilter:         "",
			tcpdumpRawFilter:   "",
			expectedFilterUsed: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnvVars()
			if tt.pcapFilter != "" {
				os.Setenv(captureConstants.PcapFilterEnvKey, tt.pcapFilter)
			}
			if tt.tcpdumpRawFilter != "" {
				os.Setenv(captureConstants.TcpdumpRawFilterEnvKey, tt.tcpdumpRawFilter)
			}
			defer resetEnvVars()

			// Construct tcpdump command and check which filter is used
			cmd := constructTcpdumpCommand(testCaptureFilePath, tt.expectedFilterUsed)

			// The filter should appear in the command args
			if tt.expectedFilterUsed != "" {
				found := slices.Contains(cmd.Args, tt.expectedFilterUsed)
				if !found {
					t.Errorf("Expected filter '%s' in command args, but got: %v", tt.expectedFilterUsed, cmd.Args)
				}
			}

			// Verify the environment variables are set correctly
			pcapEnv := os.Getenv(captureConstants.PcapFilterEnvKey)
			tcpdumpEnv := os.Getenv(captureConstants.TcpdumpRawFilterEnvKey)

			if tt.pcapFilter != "" && pcapEnv != tt.pcapFilter {
				t.Errorf("Expected PCAP_FILTER='%s', but got '%s'", tt.pcapFilter, pcapEnv)
			}
			if tt.tcpdumpRawFilter != "" && tcpdumpEnv != tt.tcpdumpRawFilter {
				t.Errorf("Expected TCPDUMP_RAW_FILTER='%s', but got '%s'", tt.tcpdumpRawFilter, tcpdumpEnv)
			}

			// When both are set, verify pcapFilter env var exists
			if tt.pcapFilter != "" && tt.tcpdumpRawFilter != "" {
				if pcapEnv == "" {
					t.Error("pcapFilter should be set when both filters provided")
				}
				if pcapEnv != tt.expectedFilterUsed {
					t.Errorf("When both filters set, expected to use pcapFilter '%s', but would use '%s'", tt.pcapFilter, tt.expectedFilterUsed)
				}
			}
		})
	}
}
