package flow_test

import (
	"context"
	"errors"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// # Data Flow via `Input` and `Output`
//
// After connected Steps into Workflow via dependencies,
// there is a very common scenarios that passing value / data through dependency.
//
// `flow` is designed with the support of flowing data between Steps, introduce `Input`:
//
//	Step(someTask).
//		DependsOn(upstreamTask).
//		Input(func(_ context.Context, someTask *SomeTask) error {
//			// fill someTask with data that
//			// only available at runtime
//			someTask.Input = upstreamTask.Output
//		}).Output(func(_ context.Context, someTask *SomeTask) error {
//			// get output from someTask
//			use(someTask.Output)
//		}),
//
// Notice the callbacks declares in Input() and Output() are executed at runtime, before Do, and per try.
func ExampleAddStep_Input() {
	// Now, let's connect the Steps into Workflow with data flow.
	var (
		workflow = new(flow.Workflow)
		imBob    = new(ImBob)
		sayHello = new(SayHello)
	)

	workflow.Add(
		flow.Step(sayHello).DependsOn(imBob).
			Input(func(ctx context.Context, sayHello *SayHello) error {
				sayHello.Who = imBob.Output // imBob's Output will be passed to sayHello's Input
				return nil
			}),
		// Notice the Input callback signature, the second parameter is the Step itself.
		// This design is intended to make the Input callback more flexible and reusable.
	)
	andAlice := func(ctx context.Context, anySayHello *SayHello) error {
		anySayHello.Who += " and Alice"
		return nil
	}
	workflow.Add(
		flow.Step(sayHello).Input(andAlice),
	)

	_ = workflow.Do(context.TODO())
	fmt.Println(sayHello.Output == "Hello Bob and Alice")
	// Output:
	// Hello Bob and Alice
	// true
}

// # BeforeStep and AfterStep callbacks
//
// [READ BELOW ONLY WHEN YOU ARE INTERESTED IN THE IMPLEMENTATION]
//
// The Input callbacks are actually a special BeforeStep callbacks.
// The BeforeStep and AfterStep callbacks are a feature that allows you to hook into the execution of a Step.
//
//	                   ▼
//	  Step           │ctx│
//	┌────────────────┘ │ └────────────────────┐
//	│                  ▼                      │
//	│          ┌────► ctx                     │
//	│          │       │                      │
//	│          │  ┌────▼─────┐                │
//	│ err==nil │  │BeforeStep├┐               │
//	│          │  └┬─────────┼│               │
//	│          │   └───┼──────┘               │
//	│          │       │                      │
//	│          │       ▼       ┌────────┐     │
//	│          └── ctx, error ─►err!=nil├─┐   │
//	│                  │       └────────┘ │   │
//	│        finish all│BeforeStep        │   │
//	│                  │                  │   │
//	│                 ctx                 │   │
//	│                  │                  │   │
//	│           ┌──────▼──────┐           │   └──
//	│           │Do(ctx) error│           ├─► err ►
//	│           └──────┬──────┘           │   ┌──
//	│                  │                  │   │
//	│              ctx,│error             │   │
//	│                  │                  │   │
//	│             ┌────▼────┐             │   │
//	│             │AfterStep├┐            │   │
//	│             └┬────────┼┼────err─────┘   │
//	│              └─────────┘                │
//	│        finish all AfterStep             │
//	└─────────────────────────────────────────┘
func ExampleAddSteps_BeforeStep() {
	workflow := new(flow.Workflow)

	var (
		foo = new(Foo)
		bar = new(Bar)
	)

	workflow.Add(
		flow.Step(foo).DependsOn(bar).
			BeforeStep(func(ctx context.Context, _ flow.Steper) (context.Context, error) {
				fmt.Println("BeforeStep")
				ctx = context.WithValue(ctx, "key", "value") // the value is available in Do
				return ctx, nil
			}).
			AfterStep(func(ctx context.Context, _ flow.Steper, err error) error {
				fmt.Println("AfterStep")
				// do some check on err
				if err != nil {
					fmt.Println("AfterStep: ", err)
				}
				return fmt.Errorf("NewError")
			}),
	)

	var errWorkflow flow.ErrWorkflow
	if errors.As(workflow.Do(context.TODO()), &errWorkflow) {
		fmt.Println(errWorkflow[foo].Unwrap())
	}
	// Output:
	// Bar
	// BeforeStep
	// Foo
	// AfterStep
	// NewError
}

type SayHello struct {
	Who    string
	Output string
}

func (s *SayHello) Do(context.Context) error {
	s.Output = "Hello " + s.Who
	fmt.Println(s.Output)
	return nil
}

type ImBob struct {
	Output string
}

func (i *ImBob) Do(context.Context) error {
	i.Output = "Bob"
	return nil
}
