package loader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

const (
	// DefaultVmlinuxHeaderDir is the default runtime include directory used by plugins.
	DefaultVmlinuxHeaderDir = "/tmp/retina/include"
	// VmlinuxHeaderDirEnv is an optional env var to override the runtime include directory.
	VmlinuxHeaderDirEnv = "RETINA_VMLINUX_HEADER_DIR"
	// VmlinuxHeaderFileName is the generated runtime BTF header filename.
	VmlinuxHeaderFileName = "vmlinux.h"
)

// VmlinuxHeaderDir returns the runtime directory where vmlinux.h is expected.
func VmlinuxHeaderDir() string {
	dir := strings.TrimSpace(os.Getenv(VmlinuxHeaderDirEnv))
	if dir == "" {
		return DefaultVmlinuxHeaderDir
	}

	return dir
}

// VmlinuxHeaderPath returns the full runtime path to vmlinux.h.
func VmlinuxHeaderPath() string {
	return filepath.Join(VmlinuxHeaderDir(), VmlinuxHeaderFileName)
}

// PrepareVmlinuxH ensures runtime vmlinux.h exists and returns the include dir.
func PrepareVmlinuxH(ctx context.Context) (string, error) {
	headerDir := VmlinuxHeaderDir()
	headerPath := filepath.Join(headerDir, VmlinuxHeaderFileName)

	if info, err := os.Stat(headerPath); err == nil {
		if info.Size() > 0 {
			return headerDir, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return headerDir, fmt.Errorf("failed to stat %s: %w", headerPath, err)
	}

	if err := GenerateVmlinuxH(ctx, headerDir); err != nil {
		return headerDir, err
	}

	return headerDir, nil
}

func GenerateVmlinuxH(ctx context.Context, outputDir string) error {
	l := log.Logger().Named("vmlinux-generator")
	vmlinuxPath := filepath.Join(outputDir, VmlinuxHeaderFileName)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if vmlinux.h already exists to avoid regenerating it unnecessarily?
	// However, if the pod restarts on a different node (unlikely for same pod instance but possible if volume persisted?),
	// or if we want to be sure.
	// Given the startup time is not critical and it's fast, let's generate it.
	// But we can check if it exists and is non-empty.
	// For now, let's just generate it.

	cmd := exec.CommandContext(ctx, "bpftool", "btf", "dump", "file", "/sys/kernel/btf/vmlinux", "format", "c")

	outfile, err := os.Create(vmlinuxPath)
	if err != nil {
		return fmt.Errorf("failed to create vmlinux.h: %w", err)
	}
	defer outfile.Close()

	cmd.Stdout = outfile
	// Capture stderr for debugging
	cmd.Stderr = os.Stderr

	l.Info("Generating vmlinux.h", zap.String("path", vmlinuxPath))
	if err := cmd.Run(); err != nil {
		l.Error("Failed to generate vmlinux.h", zap.Error(err))
		// If bpftool fails (e.g. /sys/kernel/btf/vmlinux doesn't exist), we might want to fallback or error out.
		// If it fails, the compilation will likely fail later if we rely on this header.
		return fmt.Errorf("failed to run bpftool: %w", err)
	}

	return nil
}
