// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package load

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// Kind loads images directly onto Kind cluster nodes,
// bypassing any container registry.
type Kind struct {
	ClusterName string
}

func (k *Kind) Load(ctx context.Context, images []string) error {
	provider := cluster.NewProvider()
	allNodes, err := provider.ListNodes(k.ClusterName)
	if err != nil {
		return fmt.Errorf("listing kind nodes: %w", err)
	}
	if len(allNodes) == 0 {
		return fmt.Errorf("no nodes found for kind cluster %q", k.ClusterName)
	}

	for _, image := range images {
		log.Printf("loading image %s onto kind cluster %q", image, k.ClusterName)
		if err := loadImage(ctx, allNodes, image); err != nil {
			return fmt.Errorf("loading image %s: %w", image, err)
		}
	}
	return nil
}

func (k *Kind) PullPolicy() string {
	return "IfNotPresent"
}

func (k *Kind) PullSecrets() []map[string]interface{} {
	return nil
}

func loadImage(ctx context.Context, clusterNodes []nodes.Node, image string) error {
	cmd := exec.CommandContext(ctx, "docker", "save", image)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	archive, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("docker save: %w", err)
	}

	for _, n := range clusterNodes {
		if err := nodeutils.LoadImageArchive(n, archive); err != nil {
			return fmt.Errorf("loading onto node %s: %w", n, err)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("docker save failed: %s: %w", stderr.String(), err)
	}
	return nil
}
