// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package capture

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	// InstallRetinaBinaryDir is the directory where the kubectl-retina binary will be installed
	InstallRetinaBinaryDir = "/tmp/retina-bin"
)

// InstallRetinaPlugin builds and installs the kubectl-retina plugin
// to allow e2e tests to run kubectl retina commands.
type InstallRetinaPlugin struct{}

// Run builds the kubectl-retina binary and adds it to PATH
func (i *InstallRetinaPlugin) Run() error {
	log.Print("Building kubectl-retina plugin...")

	// Create binary directory if it doesn't exist
	if err := os.MkdirAll(InstallRetinaBinaryDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create binary directory")
	}

	binaryName := "kubectl-retina"

	// Run git rev-parse to find the repository root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel") // #nosec
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to detect git repository root. Make sure you're running inside a git repository")
	}
	retinaRepoRoot := strings.TrimSpace(string(output))
	log.Printf("Auto-detected repository root: %s", retinaRepoRoot)

	_, err = os.Stat(retinaRepoRoot)
	if err != nil {
		return errors.Wrap(err, "invalid RetinaRepoRoot path")
	}

	// Check if the cli/main.go file exists
	_, err = os.Stat(filepath.Join(retinaRepoRoot, "cli", "main.go"))
	if err != nil {
		return errors.Wrap(err, "cli/main.go not found in repository root")
	}

	// Build the kubectl-retina binary
	buildCmd := exec.Command("go", "build", "-o",
		filepath.Join(InstallRetinaBinaryDir, binaryName),
		filepath.Join(retinaRepoRoot, "cli", "main.go")) // #nosec

	buildCmd.Dir = retinaRepoRoot
	var buildOutput []byte
	buildOutput, err = buildCmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to build kubectl-retina: %s", buildOutput)
	}
	log.Printf("Successfully built kubectl-retina: %s", buildOutput)

	// Add the binary directory to PATH
	currentPath := os.Getenv("PATH")

	// Check if the directory is already in PATH
	if !strings.Contains(currentPath, InstallRetinaBinaryDir) {
		newPath := fmt.Sprintf("%s:%s", InstallRetinaBinaryDir, currentPath)
		err = os.Setenv("PATH", newPath)
		if err != nil {
			return errors.Wrap(err, "failed to update PATH environment variable")
		}
		log.Printf("Added %s to PATH", InstallRetinaBinaryDir)
	}

	// Verify the plugin is accessible via kubectl
	verifyCmd := exec.Command("kubectl", "plugin", "list") // #nosec
	verifyOutput, err := verifyCmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: kubectl plugin list command failed: %v. Output: %s", err, verifyOutput)
	} else {
		log.Printf("kubectl plugin list output: %s", verifyOutput)
		if !strings.Contains(string(verifyOutput), "retina") {
			log.Printf("Warning: retina plugin not found in kubectl plugin list output")
		}
	}

	return nil
}

// Prevalidate validates the inputs before running
func (i *InstallRetinaPlugin) Prevalidate() error {
	// Check if the repository root exists

	return nil
}

// Stop is a no-op for this step
func (i *InstallRetinaPlugin) Stop() error {
	return nil
}
