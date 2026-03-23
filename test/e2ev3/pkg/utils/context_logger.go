// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"context"
	"log/slog"

	"github.com/microsoft/retina/test/e2ev3/pkg/stepname"
)

// StepLogger re-exports stepname.StepLogger so callers in the utils package
// (and workflow files that already import utils) don't need a separate import.
func StepLogger(ctx context.Context, s any) (context.Context, *slog.Logger) {
	return stepname.StepLogger(ctx, s)
}

// Prefix re-exports stepname.Prefix.
func Prefix(ctx context.Context) string {
	return stepname.Prefix(ctx)
}

// Scenario re-exports stepname.Scenario.
type Scenario = stepname.Scenario
