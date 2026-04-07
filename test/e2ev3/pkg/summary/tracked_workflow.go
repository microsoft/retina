// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package summary

import (
	"context"
	"fmt"
	"time"

	flow "github.com/Azure/go-workflow"
)

// TrackedWorkflow wraps a workflow step and records its outcome in a TestSummary.
type TrackedWorkflow struct {
	Inner   flow.Steper
	Summary *TestSummary
}

func (t *TrackedWorkflow) String() string {
	return fmt.Sprintf("%s", t.Inner)
}

func (t *TrackedWorkflow) Do(ctx context.Context) error {
	start := time.Now()
	err := t.Inner.Do(ctx)
	dur := time.Since(start)

	status := Passed
	if err != nil {
		status = Failed
	}
	t.Summary.AddWorkflow(t.String(), status, dur, err)
	return err
}
