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
	cmd := exec.CommandContext(ctx, "powershell", "-command", "$([Environment]::OSVersion).VersionString")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get windows kernel version: %s", string(output))
	}
	return strings.TrimSuffix(string(output), "\r\n"), nil
}
