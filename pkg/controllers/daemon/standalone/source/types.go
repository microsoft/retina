// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package source

import "github.com/microsoft/retina/pkg/common"

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -destination=mock_source.go  -copyright_file=../lib/ignore_headers.txt -package=source github.com/microsoft/retina/pkg/controllers/daemon/standalone/source Source

type Source interface {
	// GetAllEndpoints retrieves all retina endpoints from its corresponding source
	GetAllEndpoints() ([]*common.RetinaEndpoint, error)
}
