// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"context"
	"fmt"
	"log"

	"sigs.k8s.io/kind/pkg/cluster"
)

// CreateCluster is a go-workflow step that creates a Kind cluster
// using the native Kind Go SDK.
type CreateCluster struct {
	Config *Config
}

func (c *CreateCluster) Do(ctx context.Context) error {
	provider := cluster.NewProvider()

	clusters, err := provider.List()
	if err != nil {
		return fmt.Errorf("listing Kind clusters: %w", err)
	}
	for _, name := range clusters {
		if name == c.Config.ClusterName {
			log.Printf("Kind cluster %q already exists, skipping creation", c.Config.ClusterName)
			return nil
		}
	}

	log.Printf("creating Kind cluster %q...", c.Config.ClusterName)

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

	log.Printf("Kind cluster %q created successfully", c.Config.ClusterName)
	return nil
}
