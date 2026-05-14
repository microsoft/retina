package loader

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestGenerateVmlinuxH(t *testing.T) {
	// Initialize logger for test
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	// Create a temporary directory for output
	tmpDir, err := os.MkdirTemp("", "vmlinux-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	err = GenerateVmlinuxH(ctx, tmpDir)
	require.NoError(t, err)

	// Check if vmlinux.h exists
	vmlinuxPath := filepath.Join(tmpDir, "vmlinux.h")
	info, err := os.Stat(vmlinuxPath)
	require.NoError(t, err)
	require.False(t, info.IsDir())
	require.Positive(t, info.Size())

	// Optional: Check content starts with typedef or similar C code
	content, err := os.ReadFile(vmlinuxPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "typedef")
}

func TestVmlinuxHeaderDir_Security(t *testing.T) {
	// Initialize logger for test
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	tests := []struct {
		name     string
		envValue string
		expected string
		desc     string
	}{
		{
			name:     "default when empty",
			envValue: "",
			expected: DefaultVmlinuxHeaderDir,
			desc:     "Should use default when env var is empty",
		},
		{
			name:     "valid path",
			envValue: "/custom/path/headers",
			expected: "/custom/path/headers",
			desc:     "Should accept valid path without special characters",
		},
		{
			name:     "valid path with dashes",
			envValue: "/tmp/fake-config",
			expected: "/tmp/fake-config",
			desc:     "Should accept valid path with dashes in the middle",
		},
		{
			name:     "valid path with multiple dashes",
			envValue: "/opt/my-app/retina-headers",
			expected: "/opt/my-app/retina-headers",
			desc:     "Should accept valid path with multiple dashes",
		},
		{
			name:     "block space injection",
			envValue: "/tmp/fake --config /etc/passwd",
			expected: DefaultVmlinuxHeaderDir,
			desc:     "Should reject path with spaces (argument injection attempt)",
		},
		{
			name:     "block leading dash (flag injection)",
			envValue: "--config=/etc/passwd",
			expected: DefaultVmlinuxHeaderDir,
			desc:     "Should reject path starting with dash (would be interpreted as flag)",
		},
		{
			name:     "block config flag",
			envValue: "/tmp/dir --config /etc/shadow",
			expected: DefaultVmlinuxHeaderDir,
			desc:     "Should block clang --config file leak attack",
		},
		{
			name:     "block tab injection",
			envValue: "/tmp/fake\t--config\t/etc/shadow",
			expected: DefaultVmlinuxHeaderDir,
			desc:     "Should reject path with tabs",
		},
		{
			name:     "block newline injection",
			envValue: "/tmp/fake\n--config /etc/passwd",
			expected: DefaultVmlinuxHeaderDir,
			desc:     "Should reject path with newlines",
		},
		{
			name:     "trimmed whitespace valid",
			envValue: "  /custom/path  ",
			expected: "/custom/path",
			desc:     "Should trim leading/trailing whitespace from valid paths",
		},
		{
			name:     "block multiple arguments",
			envValue: "/tmp -I/etc -w /tmp/out.txt",
			expected: DefaultVmlinuxHeaderDir,
			desc:     "Should block attempts to inject multiple clang arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				err := os.Setenv(VmlinuxHeaderDirEnv, tt.envValue)
				require.NoError(t, err)
				defer os.Unsetenv(VmlinuxHeaderDirEnv)
			} else {
				os.Unsetenv(VmlinuxHeaderDirEnv)
			}

			// Call function and verify result
			result := VmlinuxHeaderDir()
			require.Equal(t, tt.expected, result, tt.desc)
		})
	}
}

func TestVmlinuxHeaderDir_PreventArgumentInjection(t *testing.T) {
	// Initialize logger for test
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	// Simulate attack: inject --config flag to leak /etc/passwd
	maliciousInput := "/tmp/headers --config /etc/passwd"
	err = os.Setenv(VmlinuxHeaderDirEnv, maliciousInput)
	require.NoError(t, err)
	defer os.Unsetenv(VmlinuxHeaderDirEnv)

	// Get the directory (should be sanitized to default)
	dir := VmlinuxHeaderDir()

	// Verify attack was blocked
	require.Equal(t, DefaultVmlinuxHeaderDir, dir, "Malicious input should be rejected")
	require.NotContains(t, dir, "--config", "Result should not contain injected flag")
	require.NotContains(t, dir, " ", "Result should not contain spaces")
}
