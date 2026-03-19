package flow

import (
	"fmt"
	"log/slog"
	"strings"
)

// # What is a Composite Step?
//
// Consider this case, Alice writes a Step implementation,
//
//	type DoSomeThing struct{}
//	func (d *DoSomeThing) Do(context.Context) error { /* do fancy things */ }
//
// After that, Bob finds the above implementation is useful, but still not enough.
// So Bob combines the above Steps into a new Step,
//
//	type DoManyThings struct {
//		DoSomeThing
//		DoOtherThing
//	}
//	func (d *DoManyThings) Do(context.Context) error { /* do fancy things then other thing */ }
//
// Let's call the above DoManyThings a Composite Step, the below Decorator is another example.
//
//	type Decorator struct { Steper }
//	func (d *Decorator) Do(ctx context.Context) error {
//		/* do something before */
//		err := d.Steper.Do(ctx)
//		/* do something after */
//		return err
//	}
//
// Since Workflow only requires a Step to satisfy the below interface:
//
//	type Steper interface {
//		Do(context.Context) error
//	}
//
// It's easy, intuitive, flexible and yet powerful to use Composite Steps.
//
// Actually, Workflow itself also implements Steper interface,
// meaning you can use Workflow as a Step in another Workflow!

// # How to audit / retrieve / update all steps from the Workflow?
//
//	workflow := func() *Workflow {
//		...
//		workflow.Add(Step(doSomeThing))
//		return workflow
//	}
//
//	from now on, we don't have reference to the internal steps in Workflow directly, like doSomeThing
//	however, it's totally possible have necessary to update doSomeThing,
//	like modify its input, configuration, or even its behavior (by decorator).
//
// # Introduce Unwrap()
//
// Kindly remind that, this nesting problem is not a new issue in Go.
// In Go, we have a very common error pattern:
//
//	type MyError struct { Err error }
//	func (e *MyError) Error() string { return fmt.Sprintf("MyError(%v)", e.Err) }
//
// The solution is using Unwrap() method:
//
//	func (e *MyError) Unwrap() error { return e.Err }
//
// Then standard package errors provides Is() and As() functions to help us deal with warped errors.
// We also provides a similar Has() and As() functions for Steper.
//
// Users only need to implement the below methods for your Step implementations:
//
//	type WrapStep struct { Steper }
//	func (w *WrapStep) Unwrap() Steper { return w.Steper }
//	// or
//	type WrapSteps struct { Steps []Steper }
//	func (w *WrapSteps) Unwrap() []Steper { return w.Steps }
//
// to expose your inner Steps.

type TraverseDecision int

const (
	TraverseContinue  = iota // TraverseContinue continue the traversal
	TraverseStop             // TraverseStop stop and exit the traversal immediately
	TraverseEndBranch        // TraverseEndBranch end the current branch, but continue sibling branches
)

// Traverse performs a pre-order traversal of the tree of step.
func Traverse(s Steper, f func(Steper, []Steper) TraverseDecision, walked ...Steper) TraverseDecision {
	if f == nil {
		return TraverseStop
	}
	for {
		if s == nil {
			return TraverseEndBranch
		}
		if dec := f(s, walked); dec != TraverseContinue {
			return dec
		}
		walked = append(walked, s)
		switch u := s.(type) {
		case interface{ Unwrap() Steper }:
			s = u.Unwrap()
		case interface{ Unwrap() []Steper }:
			for _, s := range u.Unwrap() {
				if dec := Traverse(s, f, walked...); dec == TraverseStop {
					return dec
				}
			}
			return TraverseContinue
		default:
			return TraverseContinue
		}
	}
}

// Has reports whether there is any step inside matches target type.
func Has[T Steper](s Steper) bool {
	find := false
	Traverse(s, func(s Steper, walked []Steper) TraverseDecision {
		if _, ok := s.(T); ok {
			find = true
			return TraverseStop
		}
		return TraverseContinue
	})
	return find
}

// As finds all steps in the tree of step that matches target type, and returns them.
// The sequence of the returned steps is pre-order traversal.
func As[T Steper](s Steper) []T {
	var rv []T
	Traverse(s, func(s Steper, walked []Steper) TraverseDecision {
		if v, ok := s.(T); ok {
			rv = append(rv, v)
		}
		return TraverseContinue
	})
	return rv
}

// HasStep reports whether there is any step matches target step.
func HasStep(step, target Steper) bool {
	if target == nil {
		return false
	}
	find := false
	Traverse(step, func(s Steper, walked []Steper) TraverseDecision {
		if s == target {
			find = true
			return TraverseStop
		}
		return TraverseContinue
	})
	return find
}

// String unwraps step and returns a proper string representation.
func String(step Steper) string {
	if step == nil {
		return "<nil>"
	}
	switch u := step.(type) {
	case interface{ String() string }:
		return u.String()
	case interface{ Unwrap() Steper }:
		return fmt.Sprintf("%T(%p) {\n\t%s\n}", u, u, indent(String(u.Unwrap())))
	case interface{ Unwrap() []Steper }:
		stepStrs := []string{}
		for _, step := range u.Unwrap() {
			stepStrs = append(stepStrs, String(step))
		}
		return fmt.Sprintf("%T(%p) {\n\t%s\n}", u, u, indent(strings.Join(stepStrs, "\n")))
	default:
		return fmt.Sprintf("%T(%p)", step, step)
	}
}

// LogValue is used with log/slog, you can use it like:
//
//	logger.With("step", LogValue(step))
//
// To prevent expensive String() calls,
//
//	logger.With("step", String(step))
func LogValue(step Steper) logValue { return logValue{Steper: step} }

type logValue struct{ Steper }

func (lv logValue) String() string       { return String(lv.Steper) }
func (lv logValue) LogValue() slog.Value { return slog.StringValue(String(lv.Steper)) }
func (lv logValue) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", String(lv.Steper))), nil
}
