// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"sigs.k8s.io/kind/pkg/cluster"
)

const kubeConfigPerms = 0o600

// ExportKubeConfig is a go-workflow step that exports the kubeconfig
// for a Kind cluster to a file using the native Kind Go SDK.
type ExportKubeConfig struct {
	ClusterName        string
	KubeConfigFilePath string
	Log                *slog.Logger
}

func (e *ExportKubeConfig) String() string { return "export-kind-kubeconfig" }

func (e *ExportKubeConfig) Do(_ context.Context) error {
	log := e.Log
	if log == nil {
		log = slog.Default()
	}
	log = log.With("step", e.String())
	log.Info("exporting kubeconfig for Kind cluster", "cluster", e.ClusterName, "path", e.KubeConfigFilePath)

	provider := cluster.NewProvider()

	kubeConfig, err := provider.KubeConfig(e.ClusterName, false)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig for Kind cluster %q: %w", e.ClusterName, err)
	}

	if err := os.WriteFile(e.KubeConfigFilePath, []byte(kubeConfig), kubeConfigPerms); err != nil {
		return fmt.Errorf("failed to write kubeconfig to %q: %w", e.KubeConfigFilePath, err)
	}

	log.Info("kubeconfig for Kind cluster written", "cluster", e.ClusterName, "path", e.KubeConfigFilePath)
	return nil
}
