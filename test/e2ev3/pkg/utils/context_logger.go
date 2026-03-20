// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import "context"

type workflowKey struct{}

// WithWorkflow stores the current workflow name in ctx.
// The StepHandler reads this to produce [workflow/step] prefixes.
func WithWorkflow(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, workflowKey{}, name)
}

// WorkflowName returns the workflow name stored in ctx, or "" if none.
func WorkflowName(ctx context.Context) string {
	if v, ok := ctx.Value(workflowKey{}).(string); ok {
		return v
	}
	return ""
}
