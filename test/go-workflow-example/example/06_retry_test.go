package flow_test

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// # Retry
//
// Workflow can retry a Step when it fails, and accept a RetryOption to customize the retry behavior.
//
//	// RetryOption customizes retry behavior of a Step in Workflow.
//	type RetryOption struct {
//		TimeoutPerTry time.Duration // 0 means no timeout
//		Attempts      uint64        // 0 means no limit
//		StopIf        func(ctx context.Context, attempt uint64, since time.Duration, err error) bool
//		Backoff       backoff.BackOff
//		Notify        backoff.Notify
//		Timer         backoff.Timer
//	}

func ExampleAddSteps_Retry() {
	var (
		workflow   = new(flow.Workflow)
		passAfter2 = &PassAfter{Attempt: 2}
	)

	workflow.Add(
		flow.Step(passAfter2).
			Retry(func(ro *flow.RetryOption) {
				ro.Attempts = 5 // retry 5 times
				ro.Timer = new(testTimer)
			}),
	)

	_ = workflow.Do(context.TODO())
	// Output:
	// failed at attempt 0
	// failed at attempt 1
	// succeed at attempt 2
}

// PassAfter keeps failing until the attempt reaches the given number.
type PassAfter struct {
	Attempt int
	count   int
}

func (p *PassAfter) Do(ctx context.Context) error {
	defer func() { p.count++ }()
	if p.count >= p.Attempt {
		fmt.Printf("succeed at attempt %d\n", p.count)
		return nil
	}
	err := fmt.Errorf("failed at attempt %d", p.count)
	fmt.Println(err)
	return err
}
