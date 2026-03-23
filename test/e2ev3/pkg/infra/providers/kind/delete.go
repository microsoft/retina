// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"context"
	"fmt"

	"github.com/microsoft/retina/test/e2ev3/pkg/stepname"
	"sigs.k8s.io/kind/pkg/cluster"
)

// DeleteCluster is a go-workflow step that deletes a Kind cluster
// using the native Kind Go SDK.
type DeleteCluster struct {
	ClusterName        string
	KubeConfigFilePath string
}

func (d *DeleteCluster) String() string { return "delete-kind-cluster" }

func (d *DeleteCluster) Do(ctx context.Context) error {
	_, log := stepname.StepLogger(ctx, d)
	log.Info("deleting Kind cluster", "cluster", d.ClusterName)

	provider := cluster.NewProvider()

	if err := provider.Delete(d.ClusterName, d.KubeConfigFilePath); err != nil {
		return fmt.Errorf("failed to delete Kind cluster %q: %w", d.ClusterName, err)
	}

	log.Info("Kind cluster deleted successfully", "cluster", d.ClusterName)
	return nil
}
