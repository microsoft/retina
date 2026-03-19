package flow_test

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/stretchr/testify/assert"
)

var (
	failedStep        = flow.Func("Failed", func(ctx context.Context) error { return assert.AnError })
	succeededStep     = flow.Func("Succeeded", func(ctx context.Context) error { return nil })
	canceledStep      = flow.Func("Canceled", func(ctx context.Context) error { return flow.Cancel(assert.AnError) })
	skippedStep       = flow.Func("Skipped", func(ctx context.Context) error { return flow.Skip(assert.AnError) })
	canceledNoErrStep = flow.Func("CanceledNoErr", func(ctx context.Context) error { return flow.Cancel(nil) })
	skippedNoErrStep  = flow.Func("SkippedNoErr", func(ctx context.Context) error { return flow.Skip(nil) })
)

func TestCondition(t *testing.T) {
	var (
		make = func(ctx context.Context, cond flow.Condition) func(*testing.T, flow.StepStatus, ...flow.Steper) {
			return func(t *testing.T, expect flow.StepStatus, steps ...flow.Steper) {
				t.Helper()
				ups := make(map[flow.Steper]flow.StepResult)
				for _, s := range steps {
					err := s.Do(ctx)
					ups[s] = flow.StepResult{
						Status: flow.StatusFromError(err),
						Err:    err,
					}
				}
				assert.Equal(t, expect, cond(ctx, ups))
			}
		}

		ctx      = context.Background()
		allSteps = []flow.Steper{failedStep, succeededStep, canceledStep, skippedStep, canceledNoErrStep, skippedNoErrStep}
	)
	t.Run("Always", func(t *testing.T) {
		v := make(ctx, flow.Always)
		v(t, flow.Running, allSteps...)
	})
	t.Run("AllSucceeded", func(t *testing.T) {
		v := make(ctx, flow.AllSucceeded)
		v(t, flow.Skipped, allSteps...)
		v(t, flow.Running, succeededStep)
	})
	t.Run("AnySucceeded", func(t *testing.T) {
		v := make(ctx, flow.AnySucceeded)
		v(t, flow.Running, allSteps...)
		v(t, flow.Skipped, failedStep, skippedStep, canceledStep)
		v(t, flow.Running, succeededStep)
	})
	t.Run("AllSucceededOrSkipped", func(t *testing.T) {
		v := make(ctx, flow.AllSucceededOrSkipped)
		v(t, flow.Skipped, allSteps...)
		v(t, flow.Skipped, failedStep, canceledStep)
		v(t, flow.Running, succeededStep, skippedStep, skippedNoErrStep)
	})
	t.Run("BeCanceled", func(t *testing.T) {
		v := make(ctx, flow.BeCanceled)
		v(t, flow.Skipped, allSteps...)
		v(t, flow.Skipped, canceledStep, canceledNoErrStep)
	})
	t.Run("AnyFailed", func(t *testing.T) {
		v := make(ctx, flow.AnyFailed)
		v(t, flow.Running, allSteps...)
		v(t, flow.Running, failedStep, canceledStep, skippedStep)
		v(t, flow.Skipped, succeededStep, skippedStep, canceledStep)
	})
	t.Run("ConditionOrDefault", func(t *testing.T) {
		v := make(ctx, flow.ConditionOrDefault(nil))
		v(t, flow.Skipped, allSteps...)
		v = make(ctx, flow.ConditionOrDefault(flow.Always))
		v(t, flow.Running, allSteps...)
	})

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	t.Run("Canceled Context", func(t *testing.T) {
		t.Run("Always", func(t *testing.T) {
			v := make(canceledCtx, flow.Always)
			v(t, flow.Running, allSteps...)
		})
		t.Run("AllSucceeded", func(t *testing.T) {
			v := make(canceledCtx, flow.AllSucceeded)
			v(t, flow.Canceled, allSteps...)
			v(t, flow.Canceled, succeededStep)
			v(t, flow.Canceled, succeededStep, skippedNoErrStep)
		})
		t.Run("AnySucceeded", func(t *testing.T) {
			v := make(canceledCtx, flow.AnySucceeded)
			v(t, flow.Canceled, allSteps...)
			v(t, flow.Canceled, succeededStep, skippedStep, failedStep)
		})
		t.Run("AllSucceededOrSkipped", func(t *testing.T) {
			v := make(canceledCtx, flow.AllSucceededOrSkipped)
			v(t, flow.Canceled, allSteps...)
			v(t, flow.Canceled, succeededStep, skippedStep)
		})
		t.Run("BeCanceled", func(t *testing.T) {
			v := make(canceledCtx, flow.BeCanceled)
			v(t, flow.Running, allSteps...)
			v(t, flow.Running, succeededStep)
			v(t, flow.Running, skippedStep, canceledStep)
		})
		t.Run("AnyFailed", func(t *testing.T) {
			v := make(canceledCtx, flow.AnyFailed)
			v(t, flow.Canceled, allSteps...)
			v(t, flow.Canceled, succeededStep)
		})
	})
}
