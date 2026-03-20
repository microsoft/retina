// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

const npmManifestURL = "https://raw.githubusercontent.com/Azure/azure-container-networking/master/npm/azure-npm.yaml"

// InstallNPM applies Azure Network Policy Manager to enable NetworkPolicy
// enforcement on Kind clusters.
type InstallNPM struct {
	KubeConfigFilePath string
}

func (n *InstallNPM) String() string { return "install-azure-npm" }

func (n *InstallNPM) Do(ctx context.Context) error {
	slog.Info("installing Azure NPM for NetworkPolicy enforcement")
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", npmManifestURL)
	if n.KubeConfigFilePath != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+n.KubeConfigFilePath)
	}
	cmdOut := &slogWriter{level: slog.LevelInfo, source: "kubectl-apply"}
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Azure NPM: %w", err)
	}
	cmdOut.Flush()

	// Wait for the DaemonSet to be ready.
	slog.Info("waiting for Azure NPM DaemonSet to be ready")
	waitCmd := exec.CommandContext(ctx, "kubectl", "rollout", "status", "daemonset/azure-npm",
		"-n", "kube-system", "--timeout=120s")
	if n.KubeConfigFilePath != "" {
		waitCmd.Env = append(os.Environ(), "KUBECONFIG="+n.KubeConfigFilePath)
	}
	waitOut := &slogWriter{level: slog.LevelInfo, source: "kubectl-rollout"}
	waitCmd.Stdout = waitOut
	waitCmd.Stderr = waitOut
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("Azure NPM DaemonSet not ready: %w", err)
	}
	waitOut.Flush()

	slog.Info("Azure NPM installed successfully")
	return nil
}
