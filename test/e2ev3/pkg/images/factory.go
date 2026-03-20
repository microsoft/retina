// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package images

import "github.com/microsoft/retina/test/e2ev3/pkg/images/load"

// NewLoader returns the appropriate image loader for the given provider.
func NewLoader(provider, clusterName string) Loader {
	if provider == "kind" {
		return &load.Kind{ClusterName: clusterName}
	}
	return &load.Registry{}
}
