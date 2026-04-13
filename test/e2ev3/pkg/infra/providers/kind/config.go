// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"fmt"
	"os/user"
	"time"

	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

// Config defines the configuration for a Kind cluster used in e2e tests.
type Config struct {
	ClusterName string
	NodeImage   string
	WaitForReady time.Duration

	// V1Alpha4Config is the native Kind cluster configuration.
	// If nil, a default single-node cluster is used.
	V1Alpha4Config *v1alpha4.Cluster
}

// DefaultE2EKindConfig returns the standard Kind cluster configuration for e2e testing.
func DefaultE2EKindConfig(clusterName string) *Config {
	if clusterName == "" {
		clusterName = defaultClusterName()
	}

	return &Config{
		ClusterName:  clusterName,
		WaitForReady: defaultWaitForReady,
		V1Alpha4Config: &v1alpha4.Cluster{
			Nodes: []v1alpha4.Node{
				{Role: v1alpha4.ControlPlaneRole},
				{Role: v1alpha4.WorkerRole},
			},
		},
	}
}

const defaultWaitForReady = 5 * time.Minute

func defaultClusterName() string {
	name := "retina-e2e"
	u, err := user.Current()
	if err == nil && u.Username != "" {
		username := u.Username
		if len(username) > 8 {
			username = username[:8]
		}
		name = fmt.Sprintf("retina-e2e-%s", username)
	}
	return name
}
