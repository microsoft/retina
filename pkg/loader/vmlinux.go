package loader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

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
	// VmlinuxKernelReleaseFileName stores the kernel release used to generate vmlinux.h.
	VmlinuxKernelReleaseFileName = "vmlinux.kernel.release"
	kernelReleasePath            = "/proc/sys/kernel/osrelease"
)

var (
	errKernelReleaseEmpty       = errors.New("kernel release is empty")
	errCachedKernelReleaseEmpty = errors.New("cached kernel release is empty")
)

// VmlinuxHeaderDir returns the runtime directory where vmlinux.h is expected.
func VmlinuxHeaderDir() string {
	dir := strings.TrimSpace(os.Getenv(VmlinuxHeaderDirEnv))
	if dir == "" {
		return DefaultVmlinuxHeaderDir
	}

	// Security: Prevent argument injection via environment variable
	// Block any path containing whitespace that could be used to inject
	// additional clang arguments (e.g., "--config /etc/passwd")
	// Also block paths starting with dash (would be interpreted as a flag)
	for _, r := range dir {
		if unicode.IsSpace(r) {
			log.Logger().Named("vmlinux").Warn(
				"RETINA_VMLINUX_HEADER_DIR contains whitespace, using default",
				zap.String("rejected_value", dir),
			)
			return DefaultVmlinuxHeaderDir
		}
	}

	if strings.HasPrefix(dir, "-") {
		log.Logger().Named("vmlinux").Warn(
			"RETINA_VMLINUX_HEADER_DIR starts with dash (potential flag injection), using default",
			zap.String("rejected_value", dir),
		)
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
	kernelRelease, err := currentKernelRelease()
	if err != nil {
		return headerDir, err
	}

	if info, err := os.Stat(headerPath); err == nil {
		if info.Size() > 0 {
			cachedKernelRelease, readErr := readCachedKernelRelease(headerDir)
			if readErr == nil && cachedKernelRelease == kernelRelease {
				return headerDir, nil
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return headerDir, fmt.Errorf("failed to stat %s: %w", headerPath, err)
	}

	if err := GenerateVmlinuxH(ctx, headerDir); err != nil {
		return headerDir, err
	}

	if err := writeCachedKernelRelease(headerDir, kernelRelease); err != nil {
		return headerDir, err
	}

	return headerDir, nil
}

func GenerateVmlinuxH(ctx context.Context, outputDir string) error {
	l := log.Logger().Named("vmlinux-generator")
	vmlinuxPath := filepath.Join(outputDir, VmlinuxHeaderFileName)
	start := time.Now()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

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
		return fmt.Errorf("failed to generate vmlinux.h: %w", err)
	}

	l.Info("Generated vmlinux.h", zap.String("path", vmlinuxPath), zap.Duration("duration", time.Since(start)))

	return nil
}

func currentKernelRelease() (string, error) {
	b, err := os.ReadFile(kernelReleasePath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", kernelReleasePath, err)
	}

	release := strings.TrimSpace(string(b))
	if release == "" {
		return "", fmt.Errorf("%w: %s", errKernelReleaseEmpty, kernelReleasePath)
	}

	return release, nil
}

func readCachedKernelRelease(outputDir string) (string, error) {
	metaPath := filepath.Join(outputDir, VmlinuxKernelReleaseFileName)
	b, err := os.ReadFile(metaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read cached kernel release %s: %w", metaPath, err)
	}

	release := strings.TrimSpace(string(b))
	if release == "" {
		return "", errCachedKernelReleaseEmpty
	}

	return release, nil
}

func writeCachedKernelRelease(outputDir, kernelRelease string) error {
	metaPath := filepath.Join(outputDir, VmlinuxKernelReleaseFileName)
	if err := os.WriteFile(metaPath, []byte(kernelRelease+"\n"), 0o600); err != nil {
		return fmt.Errorf("failed to write cached kernel release %s: %w", metaPath, err)
	}

	return nil
}
