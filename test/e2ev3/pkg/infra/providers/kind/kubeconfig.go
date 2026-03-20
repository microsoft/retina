// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"context"
	"fmt"
	"log"
	"os"

	"sigs.k8s.io/kind/pkg/cluster"
)

const kubeConfigPerms = 0o600

// ExportKubeConfig is a go-workflow step that exports the kubeconfig
// for a Kind cluster to a file using the native Kind Go SDK.
type ExportKubeConfig struct {
	ClusterName        string
	KubeConfigFilePath string
}

func (e *ExportKubeConfig) String() string { return "export-kind-kubeconfig" }

func (e *ExportKubeConfig) Do(_ context.Context) error {
	log.Printf("exporting kubeconfig for Kind cluster %q to %q...", e.ClusterName, e.KubeConfigFilePath)

	provider := cluster.NewProvider()

	kubeConfig, err := provider.KubeConfig(e.ClusterName, false)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig for Kind cluster %q: %w", e.ClusterName, err)
	}

	if err := os.WriteFile(e.KubeConfigFilePath, []byte(kubeConfig), kubeConfigPerms); err != nil {
		return fmt.Errorf("failed to write kubeconfig to %q: %w", e.KubeConfigFilePath, err)
	}

	log.Printf("kubeconfig for Kind cluster %q written to %q", e.ClusterName, e.KubeConfigFilePath)
	return nil
}
