package flow_test

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// Examples are a good place to ramp-up and have a quick look at this package's features.
//
// for basic usage,	   please go to 01 - 09
// for advanced usage, please go to 10 - ..

// # Step and Workflow
//
// Introduce two core concepts:
//
//   - Step
//   - Workflow
//
// Where Step is the unit of a Workflow,
// and Steps are connected with dependencies to form a Workflow (actually a Directed-Acyclic-Graph).
//
// They cooperate to provide features:
//
//   - Steps are easy to implement, just a trivial interface `Steper`
//   - Declare dependencies between Steps to form a Workflow
//   - Workflow executes Steps in a topological order
//
// Let's start with implementing a Step:
//
// To satisfy the interface of Step, just implement
//
//	type Steper interface {
//		Do(context.Context) error
//	}
func ExampleSteper_Do() {
	// Create a Workflow
	workflow := new(flow.Workflow)

	// Create Steps
	// Notice we normally use pointer to struct as Step,
	// internally Steps are stored in Workflow as map keys, so Steps need to be comparable.
	foo := new(Foo)
	bar := new(Bar)

	// Connect the Steps into the Workflow
	workflow.Add(
		flow.Step(foo).DependsOn(bar),
	)

	// As the code says, step `foo` depends on step `bar`, or `bar` happens-before `foo`.
	// In `flow` terms, we call `foo` as Downstream, `bar` as Upstream, since the flow is from Up to Down.
	// We'll cover dependency detail in next session.

	_ = workflow.Do(context.TODO())
	// Output:
	// Bar
	// Foo
}

type Foo struct{}

func (f *Foo) Do(ctx context.Context) error {
	fmt.Println("Foo")
	return nil
}

type Bar struct{}

func (b *Bar) Do(context.Context) error {
	fmt.Println("Bar")
	return nil
}
