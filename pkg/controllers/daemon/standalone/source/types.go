// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package source

import "github.com/microsoft/retina/pkg/common"

type Source interface {
	// GetAllEndpoints retrieves all retina endpoints from its corresponding source
	GetAllEndpoints() ([]*common.RetinaEndpoint, error)
}
