package flow

import (
	"context"
	"errors"
	"fmt"
)

// StepStatus describes the status of a Step.
type StepStatus string

const (
	Pending   StepStatus = ""          // Pending means the Step has not started yet.
	Running   StepStatus = "Running"   // Running means the Step is in progress.
	Failed    StepStatus = "Failed"    // Failed means the Step has terminated and failed.
	Succeeded StepStatus = "Succeeded" // Succeeded means the Step has terminated and succeeded.
	Canceled  StepStatus = "Canceled"  // Canceled means the Step has terminated and been canceled.
	Skipped   StepStatus = "Skipped"   // Skipped means the Step has terminated and been skipped.
)

// IsTerminated returns true if the StepStatus is one of the terminated states (Failed, Succeeded, Canceled, Skipped).
func (s StepStatus) IsTerminated() bool {
	switch s {
	case Failed, Succeeded, Canceled, Skipped:
		return true
	}
	return false
}

func (s StepStatus) String() string {
	switch s {
	case Pending:
		return "Pending"
	case Running, Failed, Succeeded, Canceled, Skipped:
		return string(s)
	default:
		return fmt.Sprintf("Unknown(%s)", string(s))
	}
}

// Condition is a function to determine what's the next status of Step.
// Condition makes the decision based on the status and result of all the Upstream Steps.
// Condition is only called when all Upstream Steps are terminated.
type Condition func(ctx context.Context, ups map[Steper]StepResult) StepStatus

var (
	// DefaultCondition used in workflow, defaults to AllSucceeded
	DefaultCondition Condition = AllSucceeded
	// DefaultIsCanceled is used to determine whether an error is being regarded as canceled.
	DefaultIsCanceled = func(err error) bool {
		switch {
		case errors.Is(err, context.Canceled),
			errors.Is(err, context.DeadlineExceeded),
			StatusFromError(err) == Canceled:
			return true
		}
		return false
	}
)

// Always runs the step as long as all upstream steps are terminated
func Always(context.Context, map[Steper]StepResult) StepStatus {
	return Running
}

// AllSucceeded runs the step when all upstream steps are Succeeded
func AllSucceeded(ctx context.Context, ups map[Steper]StepResult) StepStatus {
	if DefaultIsCanceled(ctx.Err()) {
		return Canceled
	}
	for _, up := range ups {
		if up.Status != Succeeded {
			return Skipped
		}
	}
	return Running
}

// AnySucceeded runs the step when any upstream step is Succeeded
func AnySucceeded(ctx context.Context, ups map[Steper]StepResult) StepStatus {
	if DefaultIsCanceled(ctx.Err()) {
		return Canceled
	}
	for _, up := range ups {
		if up.Status == Succeeded {
			return Running
		}
	}
	return Skipped
}

// AllSucceededOrSkipped runs the step when all upstream steps are Succeeded or Skipped
func AllSucceededOrSkipped(ctx context.Context, ups map[Steper]StepResult) StepStatus {
	if DefaultIsCanceled(ctx.Err()) {
		return Canceled
	}
	for _, up := range ups {
		if up.Status != Succeeded && up.Status != Skipped {
			return Skipped
		}
	}
	return Running
}

// BeCanceled runs the step only when the context is canceled
func BeCanceled(ctx context.Context, _ map[Steper]StepResult) StepStatus {
	if DefaultIsCanceled(ctx.Err()) {
		return Running
	}
	return Skipped
}

// AnyFailed runs the step when any upstream step is Failed
func AnyFailed(ctx context.Context, ups map[Steper]StepResult) StepStatus {
	if DefaultIsCanceled(ctx.Err()) {
		return Canceled
	}
	for _, up := range ups {
		if up.Status == Failed {
			return Running
		}
	}
	return Skipped
}

// ConditionOr will use defaultCond if cond is nil.
func ConditionOr(cond, defaultCond Condition) Condition {
	return func(ctx context.Context, ups map[Steper]StepResult) StepStatus {
		if cond == nil {
			return defaultCond(ctx, ups)
		}
		return cond(ctx, ups)
	}
}

// ConditionOrDefault will use DefaultCondition if cond is nil.
func ConditionOrDefault(cond Condition) Condition { return ConditionOr(cond, DefaultCondition) }
