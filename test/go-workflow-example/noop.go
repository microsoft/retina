package flow

import (
	"context"
)

type NoOpStep struct{ Name string }

// NoOp constructs a step doing nothing.
func NoOp(name string) *NoOpStep           { return &NoOpStep{Name: name} }
func (n *NoOpStep) String() string         { return n.Name }
func (*NoOpStep) Do(context.Context) error { return nil }
