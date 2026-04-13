// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package azure

import (
	"context"

	"k8s.io/client-go/rest"
)

// Cluster is a ClusterProvider for Azure Kubernetes Service clusters.
// Images are pulled from a container registry, so LoadImages is a no-op.
type Cluster struct {
	SubscriptionID string
	Location       string
	ResourceGroup  string
	Name           string
	KubeCfgPath    string
	RC             *rest.Config
}

func (a *Cluster) ClusterName() string            { return a.Name }
func (a *Cluster) KubeConfigPath() string          { return a.KubeCfgPath }
func (a *Cluster) RestConfig() *rest.Config        { return a.RC }

func (a *Cluster) LoadImages(_ context.Context, _ []string) error { return nil }
func (a *Cluster) ImagePullPolicy() string                        { return "Always" }

func (a *Cluster) ImagePullSecrets() []map[string]interface{} {
	return []map[string]interface{}{
		{"name": "acr-credentials"},
	}
}
