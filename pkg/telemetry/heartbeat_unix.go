//go:build unix

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	"context"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func KernelVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "uname", "-r")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get linux kernel version: %s", string(output))
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}
