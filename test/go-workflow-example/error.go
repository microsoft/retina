package flow

import (
	"fmt"
	"runtime"
	"strings"
)

// Succeed marks the current step as `Succeeded`, while still reports the error.
func Succeed(err error) ErrSucceed { return ErrSucceed{err} }

// Cancel marks the current step as `Canceled`, and reports the error.
func Cancel(err error) ErrCancel { return ErrCancel{err} }

// Skip marks the current step as `Skipped`, and reports the error.
func Skip(err error) ErrSkip { return ErrSkip{err} }

type ErrSucceed struct{ error }
type ErrCancel struct{ error }
type ErrSkip struct{ error }
type ErrPanic struct{ error }
type ErrBeforeStep struct{ error }

func (e ErrSucceed) Unwrap() error    { return e.error }
func (e ErrCancel) Unwrap() error     { return e.error }
func (e ErrSkip) Unwrap() error       { return e.error }
func (e ErrPanic) Unwrap() error      { return e.error }
func (e ErrBeforeStep) Unwrap() error { return e.error }

// WithStackTraces saves stack frames into error
func WithStackTraces(skip, depth int, ignores ...func(runtime.Frame) bool) func(error) error {
	return func(err error) error {
		pc := make([]uintptr, depth)
		i := runtime.Callers(skip, pc)
		pc = pc[:i]
		frames := runtime.CallersFrames(pc)
		withStackTraces := ErrWithStackTraces{Err: err}
		for {
			frame, more := frames.Next()
			if !more {
				break
			}
			isIgnored := false
			for _, ignore := range ignores {
				if ignore(frame) {
					isIgnored = true
					break
				}
			}
			if !isIgnored {
				withStackTraces.Frames = append(withStackTraces.Frames, frame)
			}
		}
		return withStackTraces
	}
}

// ErrWithStackTraces saves stack frames into error, and prints error into
//
//	error message
//
//	Stack Traces:
//		file:line
type ErrWithStackTraces struct {
	Err    error
	Frames []runtime.Frame
}

func (e ErrWithStackTraces) Unwrap() error { return e.Err }
func (e ErrWithStackTraces) Error() string {
	if st := e.StackTraces(); len(st) > 0 {
		return fmt.Sprintf("%s\n\nStack Traces:\n\t%s\n", e.Err, strings.Join(st, "\n\t"))
	}
	return e.Err.Error()
}
func (e ErrWithStackTraces) StackTraces() []string {
	stacks := make([]string, 0, len(e.Frames))
	for i := range e.Frames {
		stacks = append(stacks, fmt.Sprintf("%s:%d", e.Frames[i].File, e.Frames[i].Line))
	}
	return stacks
}

// StatusFromError gets the StepStatus from error.
func StatusFromError(err error) StepStatus {
	if err == nil {
		return Succeeded
	}
	for {
		switch typedErr := err.(type) {
		case ErrSucceed:
			return Succeeded
		case ErrCancel:
			return Canceled
		case ErrSkip:
			return Skipped
		case interface{ Unwrap() error }:
			err = typedErr.Unwrap()
		default:
			return Failed
		}
	}
}

// StepResult contains the status and error of a Step.
type StepResult struct {
	Status StepStatus
	Err    error
}

// StatusError will be printed as:
//
//	[Status]
//		error message
func (e StepResult) Error() string {
	rv := fmt.Sprintf("[%s]", e.Status)
	if e.Err != nil {
		rv += "\n\t" + indent(e.Err.Error())
	}
	return rv
}
func (e StepResult) Unwrap() error { return e.Err }

func indent(s string) string { return strings.ReplaceAll(s, "\n", "\n\t") }

// ErrWorkflow contains all errors reported from terminated Steps in Workflow.
//
// Keys are root Steps, values are its status and error.
type ErrWorkflow map[Steper]StepResult

func (e ErrWorkflow) Unwrap() []error {
	rv := make([]error, 0, len(e))
	for _, sErr := range e {
		rv = append(rv, sErr.Err)
	}
	return rv
}

// ErrWorkflow will be printed as:
//
//	Step: [Status]
//		error message
func (e ErrWorkflow) Error() string {
	var builder strings.Builder
	for step, serr := range e {
		builder.WriteString(fmt.Sprintf("%s: ", String(step)))
		builder.WriteString(fmt.Sprintln(serr.Error()))
	}
	return builder.String()
}

func (e ErrWorkflow) AllSucceeded() bool {
	for _, sErr := range e {
		if sErr.Status != Succeeded {
			return false
		}
	}
	return true
}
func (e ErrWorkflow) AllSucceededOrSkipped() bool {
	for _, sErr := range e {
		switch sErr.Status {
		case Succeeded, Skipped: // skipped step can have error to indicate why it's skipped
		default:
			return false
		}
	}
	return true
}

var ErrWorkflowIsRunning = fmt.Errorf("Workflow is running, please wait for it terminated")

// ErrCycleDependency means there is a cycle-dependency in your Workflow!!!
type ErrCycleDependency map[Steper][]Steper

func (e ErrCycleDependency) Error() string {
	depErr := make([]string, 0, len(e))
	for step, ups := range e {
		depsStr := []string{}
		for _, up := range ups {
			depsStr = append(depsStr, String(up))
		}
		depErr = append(depErr, fmt.Sprintf(
			"%s depends on [\n\t%s\n]",
			String(step), indent(strings.Join(depsStr, "\n")),
		))
	}
	return fmt.Sprintf("Cycle Dependency Error:\n\t%s", indent(strings.Join(depErr, "\n")))
}
