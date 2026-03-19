package flow_test

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/stretchr/testify/assert"
)

func TestErrCycleDependency(t *testing.T) {
	w := new(flow.Workflow).Add(
		flow.Step(succeededStep).DependsOn(succeededStep),
	)
	var errCycle flow.ErrCycleDependency
	if assert.ErrorAs(t, w.Do(context.Background()), &errCycle) {
		assert.ErrorContains(t, errCycle, "Succeeded depends on [\n\t\tSucceeded\n\t]")
	}
}
