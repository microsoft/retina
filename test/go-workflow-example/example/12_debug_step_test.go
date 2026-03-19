package flow_test

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// # How to debug a failed Step?
//
// A debug callback can be executed only when the target Steps are failed.
//
// If the debug step needs the result of the upstream steps, it can be achieved by hacking When.
type DebugStep struct {
	Upstreams map[flow.Steper]flow.StepResult
}

func (d *DebugStep) When(ctx context.Context, ups map[flow.Steper]flow.StepResult) flow.StepStatus {
	// save the upstreams for debug
	d.Upstreams = ups
	return flow.AnyFailed(ctx, ups)
}
func (d *DebugStep) Do(ctx context.Context) error {
	for up, statusErr := range d.Upstreams {
		switch {
		case flow.Has[*FailedStep](up):
			// handle the error
			fmt.Printf("[%s] %s", statusErr.Status, statusErr.Unwrap())
		}
	}
	return nil
}

func ExampleDebugStep() {
	var (
		debug    = new(DebugStep)
		failed   = new(FailedStep)
		workflow = new(flow.Workflow).Add(
			flow.Step(failed),
		)
	)
	// register the debug step
	workflow.Add(
		flow.Step(debug).
			DependsOn(failed).
			When(debug.When),
	)

	_ = workflow.Do(context.Background())
	// Output:
	// [Failed] failed!
}
