package flow_test

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// [STOP HERE FOR BASIC USAGE]

// # Adding a Step to Workflow is idempotent
//
// After a Workflow is constructed, you can still update the Steps in Workflow.
//
// Get the Steps in Workflow:
//
//	workflow.Steps()
//
// Adding a Step to Workflow is idempotent, so you can add the same Step multiple times,
// the configurations of the Step will be **merged**.
//
// So it's up to you to choose the pattern of declaring Steps.
//
//	a. declare a step with all its configurations together.
//
//	workflow.Add(
//		flow.Step(step).
//			DependsOn(...).
//			Input(...).
//			Timeout(...).
//			Retry(...),
//	)
//
//	b. declare a step multiple times, and each time configure different things.
//
//	workflow.Add(
//		// dependency
//		flow.Step(step).
//			DependsOn(...),
//		// ...
//		// input
//		flow.Step(step).
//			Input(...),
//	)
//	// or even in another Add()
//	workflow.Add(
//		flow.Step(step).
//			Timeout(...),
//	)
//
// So it's possible to update the Steps in Workflow, for example, to add a Retry to a Step,
// get the Steps in Workflow, via `workflow.Steps()`, then `Add()` them back to update the Step.
//
//	for _, step := range workflow.Steps() {
//		workflow.Add(
//			flow.Steps(step).Retry(...), // update the Step
//		)
//	}
func ExampleWorkflow_Add() {
	workflow := &flow.Workflow{}
	{ // scope foo and bar
		var (
			foo = &Foo{}
			bar = &Bar{}
		)
		workflow.Add(
			flow.Step(bar).DependsOn(foo),
		)
	}

	// from now on, we've lose reference to foo, bar
	// but still possible to update them (like add dependency)
	helloWorld := &SayHello{Who: "World!"}
	for _, step := range workflow.Steps() {
		workflow.Add(flow.Step(step).DependsOn(helloWorld))
	}

	_ = workflow.Do(context.Background())
	// Output:
	// Hello World!
	// Foo
	// Bar
}

// # Decorate a Step via "Wrapping"
//
// Step implementations can be reusable and composable.
//
// For example, you may have a "DecorateStep" that wraps another Step to alter its behavior:
//
//	type DecorateStep struct {
//		BaseStep flow.Steper
//	}
//
//	func (d *DecorateStep) Do(ctx context.Context) error {
//		// do something before
//		err := d.BaseStep.Do(ctx)
//		// do something after
//	}
//
// Here, you may notice we're having following problem:
//
// What if add a "DecorateStep" and its "BaseStep" to a Workflow simultaneously?
// Will `BaseStep.Do` being called twice?
//
//	base := &BaseStep{}
//	decorate := &DecorateStep{BaseStep: base}
//	workflow.Add(
//		Steps(base, decorate),
//	)
//
// The answer is NO, Workflow will only call `DecorateStep.Do`, so the `BaseStep.Do` will be called only once.
// While requiring the step implementation to have `Unwrap() Steper` method.
//
//	func (d *DecorateStep) Unwrap() Steper { return d.BaseStep }
//
// Maybe this will remind you of Go builtin package `errors`,
// for detail document, please check `Is` and `As` functions and `StepTree` type.
//
// Basically, the principal is
//
//	Workflow will only orchestrate the top-level Steps, and leave them to manage their inner Steps.
//
// Top-level Steps are the Steps that no other Steps wrap them.
func ExampleWrapStep() {
	var (
		w    = new(flow.Workflow)
		foo  = new(Foo)
		bar  = new(Bar)
		wbar = &WrapStep{bar}
	)
	w.Add(
		flow.Step(bar).DependsOn(foo),
		flow.Step(wbar), // wrap bar
	)
	_ = w.Do(context.Background())
	// Output:
	// Foo
	// WRAP: BEFORE
	// Bar
	// WRAP: AFTER
}

type WrapStep struct{ flow.Steper }

func (w *WrapStep) Unwrap() flow.Steper { return w.Steper }
func (w *WrapStep) Do(ctx context.Context) error {
	fmt.Println("WRAP: BEFORE")
	err := w.Steper.Do(ctx)
	fmt.Println("WRAP: AFTER")
	return err
}

// Since Go1.21, errors package support unwraping multiple errors, `flow` also supports this feature.
type MultiWrapStep struct{ Steps []flow.Steper }

func (m *MultiWrapStep) Unwrap() []flow.Steper { return m.Steps }
func (m *MultiWrapStep) Do(ctx context.Context) error {
	fmt.Println("MULTI: BEFORE")
	defer fmt.Println("MULTI: AFTER")
	for i, step := range m.Steps {
		fmt.Printf("MULTI: STEP %d\n", i)
		if err := step.Do(ctx); err != nil {
			return err
		}
	}
	return nil
}

func ExampleMultiWrapStep() {
	fooBar := &MultiWrapStep{
		Steps: []flow.Steper{
			new(Foo),
			new(Bar),
		},
	}
	fmt.Println(flow.Has[*Foo](fooBar)) // true
	fmt.Println(flow.Has[*Bar](fooBar)) // true

	// actually Workflow itself also implements `Unwrap() []Steper` method
	workflow := new(flow.Workflow).
		Add(
			flow.Step(fooBar).
				DependsOn(new(SayHello)),
		)

	// use As to unwrap specific type from Step
	for _, sayHello := range flow.As[*SayHello](workflow) {
		workflow.Add(flow.Step(sayHello).Input(func(ctx context.Context, sh *SayHello) error {
			sh.Who = "you can unwrap step!"
			return nil
		}))
	}

	_ = workflow.Do(context.Background())
	// Output:
	// true
	// true
	// Hello you can unwrap step!
	// MULTI: BEFORE
	// MULTI: STEP 0
	// Foo
	// MULTI: STEP 1
	// Bar
	// MULTI: AFTER
}
