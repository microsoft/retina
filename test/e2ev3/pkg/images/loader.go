// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package images

import "context"

// Loader provisions container images for a Kubernetes cluster.
// Implementations determine how images reach cluster nodes.
type Loader interface {
	// Load makes the given images available on cluster nodes.
	Load(ctx context.Context, images []string) error
	// PullPolicy returns the Kubernetes image pull policy.
	PullPolicy() string
	// PullSecrets returns image pull secret entries for Helm values.
	PullSecrets() []map[string]interface{}
}

// RetinaImages returns the standard Retina image references for the given coordinates.
func RetinaImages(registry, namespace, tag string) []string {
	base := registry + "/" + namespace
	return []string{
		base + "/retina-agent:" + tag,
		base + "/retina-init:" + tag,
		base + "/retina-operator:" + tag,
	}
}
