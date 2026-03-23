// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package capture

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// InstallRetinaBinaryDir is the directory where the kubectl-retina binary will be installed.
	InstallRetinaBinaryDir = "/tmp/retina-bin"
)

// InstallRetinaPluginStep builds and installs the kubectl-retina plugin
// to allow e2e tests to run kubectl retina commands.
type InstallRetinaPluginStep struct{}

func (i *InstallRetinaPluginStep) String() string { return "install-retina-plugin" }

func (i *InstallRetinaPluginStep) Do(ctx context.Context) error {
	log := slog.With("step", i.String())
	log.Info("building kubectl-retina plugin")

	if err := os.MkdirAll(InstallRetinaBinaryDir, 0o755); err != nil {
		return fmt.Errorf("failed to create binary directory: %w", err)
	}

	binaryName := "kubectl-retina"

	cmd := exec.Command("git", "rev-parse", "--show-toplevel") // #nosec
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to detect git repository root: %w", err)
	}
	retinaRepoRoot := strings.TrimSpace(string(output))
	log.Info("auto-detected repository root", "path", retinaRepoRoot)

	if _, err := os.Stat(retinaRepoRoot); err != nil {
		return fmt.Errorf("invalid RetinaRepoRoot path: %w", err)
	}

	if _, err := os.Stat(filepath.Join(retinaRepoRoot, "cli", "main.go")); err != nil {
		return fmt.Errorf("cli/main.go not found in repository root: %w", err)
	}

	buildCmd := exec.Command("go", "build", "-o",
		filepath.Join(InstallRetinaBinaryDir, binaryName),
		filepath.Join(retinaRepoRoot, "cli", "main.go")) // #nosec
	buildCmd.Dir = retinaRepoRoot
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build kubectl-retina: %s: %w", buildOutput, err)
	}
	log.Info("successfully built kubectl-retina", "output", string(buildOutput))

	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, InstallRetinaBinaryDir) {
		newPath := fmt.Sprintf("%s:%s", InstallRetinaBinaryDir, currentPath)
		if err := os.Setenv("PATH", newPath); err != nil {
			return fmt.Errorf("failed to update PATH environment variable: %w", err)
		}
		log.Info("added directory to PATH", "dir", InstallRetinaBinaryDir)
	}

	verifyCmd := exec.Command("kubectl", "plugin", "list") // #nosec
	verifyOutput, err := verifyCmd.CombinedOutput()
	if err != nil {
		log.Warn("kubectl plugin list command failed", "error", err, "output", string(verifyOutput))
	} else {
		log.Info("kubectl plugin list", "output", string(verifyOutput))
		if !strings.Contains(string(verifyOutput), "retina") {
			log.Warn("retina plugin not found in kubectl plugin list output")
		}
	}

	return nil
}
