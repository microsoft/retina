// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLinuxPlatformKernelVersion(t *testing.T) {
	InitAppInsights("", "")
	ctx := context.TODO()

	str, err := KernelVersion(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, str)
}
