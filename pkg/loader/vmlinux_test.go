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
	require.Greater(t, info.Size(), int64(0))

	// Optional: Check content starts with typedef or similar C code
	content, err := os.ReadFile(vmlinuxPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "typedef")
}
