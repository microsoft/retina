//go:build windows

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWindowsFileCheckCommandResistsInjection runs the actual cmd.exe
// FileCheckCommand shape used by getDownloadCmd against real files. Unlike
// the Linux sh -c "$1" pattern, cmd.exe has no positional-parameter-style
// protection: it re-parses its whole command line as text, so this proves
// paths built from UnsafePathChars-allowed characters (space, unicode) are
// still found correctly, and that a denylisted metacharacter, if it ever
// reached this command unfiltered, would be interpreted by cmd.exe rather
// than treated as a literal filename -- justifying why UnsafePathChars must
// reject it upstream in getDownloadCmd.
func TestWindowsFileCheckCommandResistsInjection(t *testing.T) {
	cmdPath, err := exec.LookPath("cmd")
	if err != nil {
		t.Skip("cmd.exe not available on this system")
	}

	dir := t.TempDir()
	writeFile := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("data"), 0o600); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		return p
	}

	testCases := []struct {
		name       string
		srcPath    string
		wantMarker bool
	}{
		{name: "existing readable file reports found", srcPath: writeFile("exists.txt"), wantMarker: true},
		{name: "missing file reports not found", srcPath: filepath.Join(dir, "missing.txt"), wantMarker: false},
		{name: "path with embedded space is found", srcPath: writeFile("job name.txt"), wantMarker: true},
		{name: "path with unicode is found", srcPath: writeFile("captüre.txt"), wantMarker: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// tc.srcPath is a fixed, test-controlled value (never user input).
			cmd := exec.CommandContext(t.Context(), cmdPath, "/c", "if", "exist", tc.srcPath, "echo", fileExistsMarker) // #nosec G204
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("command execution failed: %v (output: %s)", err, output)
			}

			if gotMarker := strings.Contains(string(output), fileExistsMarker); gotMarker != tc.wantMarker {
				t.Errorf("expected marker=%v, got output %q", tc.wantMarker, output)
			}
		})
	}

	// Demonstrates why '&' must stay in UnsafePathChars: unlike the Linux
	// script, cmd.exe has no mechanism that would neutralize it here.
	proofFile := filepath.Join(dir, "pwned")
	injected := filepath.Join(dir, "pwn&(echo hi>"+proofFile+")")
	cmd := exec.CommandContext(t.Context(), cmdPath, "/c", "if", "exist", injected,
		"echo", fileExistsMarker) // #nosec G204 -- shows why '&' must be denied upstream, not a reachable download path
	if output, err := cmd.CombinedOutput(); err != nil {
		// Non-zero exit is fine here; only proofFile's existence matters.
		t.Logf("cmd exited non-zero: %v (output: %s)", err, output)
	}

	if _, statErr := os.Stat(proofFile); statErr != nil {
		t.Fatal("expected the '&' payload to execute via cmd.exe, proving UnsafePathChars must reject it before this command is built")
	}
}
