package flow_test

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// # Mock Step in Workflow for unit-test
//
// When writing unit tests for a composite Step or a Workflow, it's often necessary to mock the behavior of the inner Steps.
//
// We can MockStep by wrapping the original Step and override the Do() method.
func ExampleMockStep() {
	var (
		foo = new(Foo)
		bar = new(Bar)
		w   = new(flow.Workflow)
	)
	w.Add(
		flow.Step(bar).DependsOn(foo),
		flow.Mock(foo, func(ctx context.Context) error {
			fmt.Println("MockFoo")
			return nil
		}),
	)
	_ = w.Do(context.Background())
	// Output:
	// MockFoo
	// Bar
}
