// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package stepname

import (
	"context"
	"log/slog"

	flow "github.com/Azure/go-workflow"
)

type prefixKey struct{}

// StepLogger appends the step name of s to the accumulated context prefix
// and returns the enriched context + a logger tagged with the full prefix.
//
// Call this at the top of every Do(ctx) at any level:
//
//	func (w *Workflow) Do(ctx context.Context) error {
//	    ctx, log := stepname.StepLogger(ctx, w)  // prefix = "basic-metrics"
//	    ...
//	}
//	func (p *PortForward) Do(ctx context.Context) error {
//	    _, log := stepname.StepLogger(ctx, p)    // prefix = "basic-metrics/drop/port-forward"
//	    ...
//	}
func StepLogger(ctx context.Context, s any) (context.Context, *slog.Logger) {
	name := StepName(s)
	existing := Prefix(ctx)
	var prefix string
	if existing != "" {
		prefix = existing + "/" + name
	} else {
		prefix = name
	}
	ctx = context.WithValue(ctx, prefixKey{}, prefix)
	return ctx, slog.Default().With("prefix", prefix)
}

// Prefix returns the accumulated log prefix stored in ctx.
func Prefix(ctx context.Context) string {
	if v, ok := ctx.Value(prefixKey{}).(string); ok {
		return v
	}
	return ""
}

// Scenario wraps a flow.Workflow with a name that gets added to the
// context prefix when executed. Use this for test/scenario grouping:
//
//	&stepname.Scenario{Name: "drop", Inner: buildDropWorkflow(...)}
type Scenario struct {
	Name  string
	Inner *flow.Workflow
}

func (s *Scenario) String() string { return s.Name }

func (s *Scenario) Do(ctx context.Context) error {
	ctx, _ = StepLogger(ctx, s)
	return s.Inner.Do(ctx)
}
