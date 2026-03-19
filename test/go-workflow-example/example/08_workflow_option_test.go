package flow_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	flow "github.com/Azure/go-workflow"
)

// # Workflow Options
//
// Workflow provides options that configures its behavior.
//
//	type Workflow struct {
//		MaxConcurrency int  // MaxConcurrency limits the max concurrency of running Steps
//		DontPanic      bool // DontPanic suppress panics, instead return it as error
//		OKToSkip       bool // OKToSkip returns nil if all Steps succeeded or skipped, otherwise only return nil if all Steps succeeded
//	}

func ExampleWorkflow_MaxConcurrency() {
	var (
		workflow = &flow.Workflow{
			MaxConcurrency: 2,
		}

		counter = new(atomic.Int32)
		start   = make(chan struct{})
		done    = make(chan struct{})

		countOneThenWaitDone = func(context.Context) error {
			counter.Add(1)
			start <- struct{}{} // signal start
			<-done
			return nil
		}

		a = flow.Func("a", countOneThenWaitDone)
		b = flow.Func("b", countOneThenWaitDone)
		c = flow.Func("c", countOneThenWaitDone)
	)

	workflow.Add(flow.Steps(a, b, c))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = workflow.Do(context.TODO())
	}()

	// should only two Steps are running concurrently
	<-start
	<-start
	// <-start // this will block
	fmt.Println(counter.Load()) // 2

	// unblock one Step
	done <- struct{}{}
	<-start
	fmt.Println(counter.Load()) // 3

	// unblock all Step
	close(done)

	// wait the Workflow to finish
	wg.Wait()

	// Output:
	// 2
	// 3
}

func ExampleWorkflow_DontPanic() {
	var (
		workflow = &flow.Workflow{
			DontPanic: true,
		}

		panicStep = flow.Func("panic", func(context.Context) error {
			panic("I'm panicking")
		})
	)

	workflow.Add(flow.Step(panicStep))

	fmt.Println(workflow.Do(context.TODO()))
	// Output:
	// panic: [Failed]
	//	I'm panicking
}

func ExampleWorkflow_SkipAsError() {
	var (
		workflow1 = &flow.Workflow{
			SkipAsError: true,
		}
		workflow2 = &flow.Workflow{
			SkipAsError: false,
		}

		skipped = flow.Func("skipped", func(context.Context) error {
			return flow.Skip(fmt.Errorf("skip me"))
		})
	)

	workflow1.Add(flow.Step(skipped))
	workflow2.Add(flow.Step(skipped))

	fmt.Println(workflow1.Do(context.TODO()))
	fmt.Println(workflow2.Do(context.TODO()))
	// Output:
	// skipped: [Skipped]
	//	skip me
	//
	// <nil>
}

func ExampleWorkflow_DefaultOption() {
	var (
		defaultTimeout = 10 * time.Minute
		workflow       = &flow.Workflow{
			DefaultOption: &flow.StepOption{
				Timeout: &defaultTimeout,
			},
		}
		step = flow.NoOp("step")
	)

	workflow.Add(flow.Step(step))
	opt := workflow.StateOf(step).Option()
	fmt.Println(*opt.Timeout)
	// Output:
	// 10m0s
}
