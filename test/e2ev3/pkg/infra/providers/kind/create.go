// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"context"
	"fmt"
	"log/slog"

	"sigs.k8s.io/kind/pkg/cluster"
)

// CreateCluster is a go-workflow step that creates a Kind cluster
// using the native Kind Go SDK.
type CreateCluster struct {
	Config *Config
}

func (c *CreateCluster) String() string { return "create-kind-cluster" }

func (c *CreateCluster) Do(ctx context.Context) error {
	log := slog.With("step", c.String())
	provider := cluster.NewProvider()

	clusters, err := provider.List()
	if err != nil {
		return fmt.Errorf("listing Kind clusters: %w", err)
	}
	for _, name := range clusters {
		if name == c.Config.ClusterName {
			log.Info("Kind cluster already exists, skipping creation", "cluster", c.Config.ClusterName)
			return nil
		}
	}

	log.Info("creating Kind cluster", "cluster", c.Config.ClusterName)

	opts := []cluster.CreateOption{
		cluster.CreateWithWaitForReady(c.Config.WaitForReady),
		cluster.CreateWithDisplayUsage(false),
		cluster.CreateWithDisplaySalutation(false),
	}

	if c.Config.NodeImage != "" {
		opts = append(opts, cluster.CreateWithNodeImage(c.Config.NodeImage))
	}

	if c.Config.V1Alpha4Config != nil {
		opts = append(opts, cluster.CreateWithV1Alpha4Config(c.Config.V1Alpha4Config))
	}

	if err := provider.Create(c.Config.ClusterName, opts...); err != nil {
		return fmt.Errorf("failed to create Kind cluster %q: %w", c.Config.ClusterName, err)
	}

	log.Info("Kind cluster created successfully", "cluster", c.Config.ClusterName)
	return nil
}
