package conntrack

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
)

func TestBuildDynamicHeaderPath(t *testing.T) {
	tests := []struct {
		name         string
		expectedPath string
	}{
		{
			name:         "ExpectedPath",
			expectedPath: fmt.Sprintf("%s/%s/%s", path.Dir(getCurrentFilePath(t)), bpfSourceDir, dynamicHeaderFileName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualPath := BuildDynamicHeaderPath()
			if actualPath != tt.expectedPath {
				t.Errorf("unexpected dynamic header path: got %q, want %q", actualPath, tt.expectedPath)
			}
		})
	}
}

func TestGenerateDynamic(t *testing.T) {
	tests := []struct {
		name             string
		conntrackMetrics int
		expectedContents string
	}{
		{
			name:             "ConntrackMetricsEnabled",
			conntrackMetrics: 1,
			expectedContents: "#define ENABLE_CONNTRACK_METRICS 1\n",
		},
		{
			name:             "ConntrackMetricsDisabled",
			conntrackMetrics: 0,
			expectedContents: "#define ENABLE_CONNTRACK_METRICS 0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory
			tempDir, err := os.MkdirTemp("", "conntrack_test")
			if err != nil {
				t.Fatalf("failed to create temp directory: %v", err)
			}
			// Clean up the temporary directory after the test completes
			defer os.RemoveAll(tempDir)

			// Override the dynamicHeaderPath to use the temporary directory
			dynamicHeaderPath := path.Join(tempDir, dynamicHeaderFileName)

			// Call the GenerateDynamic function and check if it returns an error.
			ctx := context.Background()
			if err = GenerateDynamic(ctx, dynamicHeaderPath, tt.conntrackMetrics); err != nil {
				t.Fatalf("failed to generate dynamic header: %v", err)
			}

			// Verify that the dynamic header file was created in the expected location and contains the expected contents.
			if _, err = os.Stat(dynamicHeaderPath); os.IsNotExist(err) {
				t.Fatalf("dynamic header file does not exist: %v", err)
			}

			actualContents, err := os.ReadFile(dynamicHeaderPath)
			if err != nil {
				t.Fatalf("failed to read dynamic header file: %v", err)
			}
			if string(actualContents) != tt.expectedContents {
				t.Errorf("unexpected dynamic header file contents: got %q, want %q", string(actualContents), tt.expectedContents)
			}
		})
	}
}

func getCurrentFilePath(t *testing.T) string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("failed to determine test file path")
	}
	return filename
}
