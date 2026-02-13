//go:build unix

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	"context"

	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
)

func KernelVersion(context.Context) (string, error) {
	release, err := utils.KernelRelease()
	if err != nil {
		return "", errors.Wrap(err, "failed to get linux kernel version")
	}
	return release, nil
}
