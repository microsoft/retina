//go:build !windows

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standard

import "context"

func setupShutdownMarker(context.Context, context.CancelFunc) {}
