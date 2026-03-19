package flow_test

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/stretchr/testify/assert"
)

func TestNoOpStep(t *testing.T) {
	noop := flow.NoOp("noop")
	assert.Equal(t, "noop", noop.String())
	assert.NoError(t, noop.Do(context.Background()))
}
