// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package steps provides go-workflow Steper implementations that wrap
// the existing e2e framework steps for use in go-workflow workflows.
package steps

import (
	"context"
	"time"
)

// SleepStep pauses execution for a given duration.
type SleepStep struct {
	Duration time.Duration
}

func (s *SleepStep) Do(ctx context.Context) error {
	select {
	case <-time.After(s.Duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
