// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package statefile

import (
	"os"

	"github.com/pkg/errors"

	"github.com/microsoft/retina/pkg/controllers/daemon/standalone/source"
	"github.com/microsoft/retina/pkg/controllers/daemon/standalone/source/statefile/azure"
)

var ErrUnsupportedStatefileType = errors.New("unsupported statefile enrichment type, valid types are: azure-vnet-statefile")

func newStatefile(enrichmentMode, location string) (source.Source, error) {
	switch enrichmentMode {
	case "azure-vnet-statefile":
		return azure.New(location), nil
	default:
		return nil, errors.Wrapf(ErrUnsupportedStatefileType, "enrichmentMode=%s", enrichmentMode)
	}
}

func New(enrichmentMode, location string) (source.Source, error) {
	if _, err := os.Stat(location); os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "statefile does not exist at location: %s", location)
	}

	return newStatefile(enrichmentMode, location)
}
