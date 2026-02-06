package loader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

func GenerateVmlinuxH(ctx context.Context, outputDir string) error {
	l := log.Logger().Named("vmlinux-generator")
	vmlinuxPath := filepath.Join(outputDir, "vmlinux.h")

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
