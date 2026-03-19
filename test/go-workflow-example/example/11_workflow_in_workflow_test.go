package flow_test

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// # Workflow in Workflow
//
// Maybe you've already noticed that, Workflow also implements Steper interface.
//
//	func (w *Workflow) Do(ctx context.Context) error
//
// Which means, you can actually put a Workflow into another Workflow as a Step!
//
// We encourage you to use this feature to build complex workflows.
func ExampleWorkflow_Do() {
	var (
		foo   = new(Foo)
		bar   = new(Bar)
		inner = new(flow.Workflow).Add(
			flow.Step(bar).DependsOn(foo),
		)

		before = Print("Before")
		after  = Print("After")
		outer  = new(flow.Workflow).Add(
			flow.Pipe(before, inner, after),
		)
	)

	_ = outer.Do(context.Background())
	// Output:
	// Before
	// Foo
	// Bar
	// After
}

type Print string

func (p Print) Do(ctx context.Context) error {
	fmt.Println(p)
	return nil
}
