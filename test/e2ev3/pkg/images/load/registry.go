// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package load

import "context"

// Registry is a Loader for clusters that pull images from a container registry.
// Load is a no-op since images are already available in the registry.
type Registry struct{}

func (r *Registry) Load(_ context.Context, _ []string) error {
	return nil
}

func (r *Registry) PullPolicy() string {
	return "Always"
}

func (r *Registry) PullSecrets() []map[string]interface{} {
	return []map[string]interface{}{
		{"name": "acr-credentials"},
	}
}
