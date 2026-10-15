//go:build linux

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
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	tmpDir, err := os.MkdirTemp("", "vmlinux-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = GenerateVmlinuxH(context.Background(), tmpDir)
	require.NoError(t, err)

	vmlinuxPath := filepath.Join(tmpDir, "vmlinux.h")
	info, err := os.Stat(vmlinuxPath)
	require.NoError(t, err)
	require.False(t, info.IsDir())
	require.Positive(t, info.Size())

	content, err := os.ReadFile(vmlinuxPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "typedef")
}
