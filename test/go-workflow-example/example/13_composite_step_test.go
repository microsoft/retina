package flow_test

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// # Composite Step
//
// Writing a Step with only a few operations is easy,
// but writing a Step that contains multiple and complex operations is challenging.
//
// We can reuse and compose simple Steps to form a composite Step.
//
// However, composite step still has few drawbacks:
//	- it's not unit-test-able
//	- the inner steps are invisible to the workflow if composite step not implement Unwrap() method
//	- only one error returned from Do(), lose detailed inner step error
//	- when add input callbacks to the inner steps, the callbacks will be called before the composite step's Do()
//
// Thus, we recommend to use Workflow-in-Workflow to build a composite step.

type Bootstrap struct{}
type Cleanup struct{}
type SimpleStep struct{ Value string }
type CompositeStep struct {
	Bootstrap
	SimpleStep
	Cleanup
}

func (b *Bootstrap) Do(ctx context.Context) error {
	fmt.Println("Bootstrap")
	return nil
}
func (c *Cleanup) Do(ctx context.Context) error {
	fmt.Println("Cleanup")
	return nil
}
func (s *SimpleStep) Do(ctx context.Context) error {
	fmt.Printf("SimpleStep: %s\n", s.Value)
	return fmt.Errorf("SimpleStep Failed!")
}
func (c *CompositeStep) String() string { return "CompositeStep" }
func (c *CompositeStep) Unwrap() []flow.Steper {
	return []flow.Steper{&c.Bootstrap, &c.SimpleStep, &c.Cleanup}
}
func (c *CompositeStep) Do(ctx context.Context) error {
	if err := c.Bootstrap.Do(ctx); err != nil {
		return err
	}
	defer c.Cleanup.Do(ctx)
	return c.SimpleStep.Do(ctx)
}

func ExampleCompositeStep() {
	workflow := new(flow.Workflow)
	workflow.Add(
		flow.Step(new(CompositeStep)).
			Input(func(ctx context.Context, cs *CompositeStep) error {
				cs.SimpleStep.Value = "Action!"
				return nil
			}),
	)
	err := workflow.Do(context.Background())
	fmt.Println(err)
	// Output:
	// Bootstrap
	// SimpleStep: Action!
	// Cleanup
	// CompositeStep: [Failed]
	// 	SimpleStep Failed!
}

func ExampleCompositeViaWorkflow() {
	var (
		composite = &CompositeViaWorkflow{SimpleStep: SimpleStep{
			Value: "Action!",
		}}
		w = new(flow.Workflow).Add(
			flow.Step(composite),
		)
	)
	_ = w.Do(context.Background())
	// Output:
	// Bootstrap
	// SimpleStep: Action!
}

type CompositeViaWorkflow struct {
	SimpleStep
	w *flow.Workflow
}

func (c *CompositeViaWorkflow) Unwrap() flow.Steper          { return c.w }
func (c *CompositeViaWorkflow) Do(ctx context.Context) error { return c.w.Do(ctx) }
func (c *CompositeViaWorkflow) BuildStep() {
	c.w = &flow.Workflow{}
	var (
		bootstrap = new(Bootstrap)
		cleanup   = new(Cleanup)
		simple    = &c.SimpleStep
	)
	c.w.Add(
		flow.Pipe(bootstrap, simple, cleanup),
	)
}
