package flow

import (
	"context"
	"iter"
	"maps"
	"slices"
	"time"
)

// Steper describes the requirement for a step, which is basic unit of a Workflow.
//
// Implement this interface to allow Workflow orchestrating your steps.
//
// Notice Steper will be saved in Workflow as map key, so it's supposed to be 'comparable' type like struct pointer.
//
// Do not use empty struct{} as Steper implementation, because all empty struct{} are equal in Go,
// which makes it impossible to distinguish different steps in Workflow.
type Steper interface {
	Do(context.Context) error
}

// Builder is an interface to add steps into Workflow
type Builder interface {
	AddToWorkflow() map[Steper]*StepConfig
}

// BeforeStep defines callback being called BEFORE step being executed.
type BeforeStep func(context.Context, Steper) (context.Context, error)

// AfterStep defines callback being called AFTER step being executed.
type AfterStep func(context.Context, Steper, error) error

// StepConfig is the configuration of a step in a Workflow.
type StepConfig struct {
	Upstreams Set[Steper]         // Upstreams of the step, means these steps should happen-before this step
	Before    []BeforeStep        // Before callbacks of the step, will be called before Do
	After     []AfterStep         // After callbacks of the step, will be called before Do
	Option    []func(*StepOption) // Option customize the step settings
}

// StepOption customizes the behavior of how Workflow orchestrates the step.
type StepOption struct {
	RetryOption *RetryOption   // RetryOption customize how the step should be retried, default (nil) means no retry.
	Condition   Condition      // Condition decides whether Workflow should execute the step, default to DefaultCondition.
	Timeout     *time.Duration // Timeout sets the step level timeout, default (nil) means no timeout.
}

// Steps bakes Steper(s) ready to be added into Workflow.
//
// The Steper(s) declared are mutually independent, meaning they will be executed in parallel.
//
//	workflow.Add(
//		Steps(a, b, c),					// a, b, c will be executed in parallel
//		Steps(a, b, c).DependsOn(d, e), // d, e will be executed in parallel, then a, b, c in parallel
//	)
func Steps(steps ...Steper) AddSteps {
	rv := make(AddSteps)
	for _, step := range steps {
		rv[step] = &StepConfig{Upstreams: make(Set[Steper])}
	}
	return rv
}

// Step bakes typed Steper(s) ready to be added into Workflow.
//
// The main difference between Step() and Steps() is that,
// Step() allows to add Input callbacks for the step (since this is a generic function).
//
//	Step(a).Input(func(ctx context.Context, a *A) error {
//		// fill a
//	}))
func Step[S Steper](steps ...S) AddStep[S] {
	return AddStep[S]{
		AddSteps: Steps(ToSteps(steps)...),
		Steps:    steps,
	}
}

// Pipe creates a pipeline in Workflow.
//
//	workflow.Add(
//		Pipe(a, b, c), // a -> b -> c
//	)
//
// The above code is equivalent to:
//
//	workflow.Add(
//		Step(b).DependsOn(a),
//		Step(c).DependsOn(b),
//	)
func Pipe(steps ...Steper) AddSteps {
	as := Steps(steps...)
	for i := 0; i < len(steps)-1; i++ {
		as.Merge(Steps(steps[i+1]).DependsOn(steps[i]))
	}
	return as
}

// BatchPipe creates a batched pipeline in Workflow.
//
//	workflow.Add(
//		BatchPipe(
//			Steps(a, b),
//			Steps(c, d, e),
//			Steps(f),
//		),
//	)
//
// The above code is equivalent to:
//
//	workflow.Add(
//		Steps(c, d, e).DependsOn(a, b),
//		Steps(f).DependsOn(c, d, e),
//	)
func BatchPipe(batch ...AddSteps) AddSteps {
	as := Steps()
	for _, other := range batch {
		as.Merge(other)
	}
	for i := 0; i < len(batch)-1; i++ {
		as.Merge(Steps(Keys(batch[i+1])...).DependsOn(Keys(batch[i])...))
	}
	return as
}

// DependsOn declares dependency on the given Steps.
//
//	Step(a).DependsOn(b, c)
//
// Then b, c should happen-before a.
func (as AddSteps) DependsOn(ups ...Steper) AddSteps {
	for down := range as {
		as[down].Upstreams.Add(ups...)
	}
	return as
}

// Input adds BeforeStep callback for the Step(s).
//
// Input callbacks will be called before Do,
// and the order will respect the order of declarations.
//
//	Step(a).
//		Input(/* 1. this Input will be called first */).
//		Input(/* 2. this Input will be called after 1. */)
//	Step(a).Input(/* 3. this Input is after all */)
//
// The Input callbacks are executed at runtime and per-try.
func (as AddStep[S]) Input(fns ...func(context.Context, S) error) AddStep[S] {
	for _, step := range as.Steps {
		step := step // capture range variable
		for _, fn := range fns {
			if fn != nil {
				fn := fn // capture range variable
				as.AddSteps[step].Before = append(as.AddSteps[step].Before, func(ctx context.Context, _ Steper) (context.Context, error) {
					return ctx, fn(ctx, step)
				})
			}
		}
	}
	return as
}

// Output can pass the results of the Step to outer scope.
// Output is only triggered when the Step is successful (returns nil error).
//
// Output actually adds AfterStep callback for the Step(s).
//
// The Output callbacks are executed at runtime and per-try.
func (as AddStep[S]) Output(fns ...func(context.Context, S) error) AddStep[S] {
	for _, step := range as.Steps {
		step := step // capture range variable
		for _, fn := range fns {
			if fn != nil {
				fn := fn // capture range variable
				as.AddSteps[step].After = append(as.AddSteps[step].After, func(ctx context.Context, _ Steper, err error) error {
					if err == nil {
						return fn(ctx, step)
					}
					return err
				})
			}
		}
	}
	return as
}

// BeforeStep adds BeforeStep callback for the Step(s).
//
// The BeforeStep callback will be called before Do, and return when first error occurs.
// The order of execution will respect the order of declarations.
// The BeforeStep callbacks are able to change the context.Context feed into Do.
// The BeforeStep callbacks are executed at runtime and per-try.
func (as AddSteps) BeforeStep(befores ...BeforeStep) AddSteps {
	for step := range as {
		as[step].Before = append(as[step].Before, befores...)
	}
	return as
}

// AfterStep adds AfterStep callback for the Step(s).
//
// The AfterStep callback will be called after Do, and pass the error to next AfterStep callback.
// The order of execution will respect the order of declarations.
// The AfterStep callbacks are able to change the error returned by Do.
// The AfterStep callbacks are executed at runtime and per-try.
//
// Tip:
//
// Remember to check error when overriding your own AfterStep function.
//
//	Steps(a).AfterStep(func(ctx context.Context, step Steper, err error) error {
//		if err != nil {
//			// do something when error happens
//		}
//		// do something when success
//		return err // you can decide whether pass through the error or not
//	})
func (as AddSteps) AfterStep(afters ...AfterStep) AddSteps {
	for step := range as {
		as[step].After = append(as[step].After, afters...)
	}
	return as
}

// Timeout sets the Step level timeout.
// Last one wins.
func (as AddSteps) Timeout(timeout time.Duration) AddSteps {
	for step := range as {
		as[step].Option = append(as[step].Option, func(so *StepOption) {
			so.Timeout = &timeout
		})
	}
	return as
}

// When set the Condition for the Step.
// Last one wins.
//
// Tip:
//
// Remember to check upstreams when overriding your own Condition function.
//
//	Steps(a).When(func(ctx context.Context, ups map[Steper]StepResult) StepStatus {
//		// check upstreams leveraging built-in Conditions
//		status := flow.AllSucceeded(ctx, ups)
//		if status != flow.Running {
//			return status // return fast if the Condition not satisfied
//		}
//		// your custom logic
//		if ... {
//			return flow.Running
//		}
//		return flow.Skipped
//	})
func (as AddSteps) When(cond Condition) AddSteps {
	for step := range as {
		as[step].Option = append(as[step].Option, func(so *StepOption) {
			so.Condition = cond
		})
	}
	return as
}

// Retry customize how the Step should be retried.
//
// Step will use DefaultRetryOption when this option is configured with nil.
//
//	w.Add(
//		Step(a), // not retry
//		Step(b).Retry(func(opt *RetryOption) { // will retry 3 times
//			opt.MaxAttempts = 3
//		}),
//		Step(c).Retry(nil), // will use DefaultRetryOption!
//	)
func (as AddSteps) Retry(opts ...func(*RetryOption)) AddSteps {
	for step := range as {
		as[step].Option = append(as[step].Option, func(so *StepOption) {
			if so.RetryOption == nil {
				so.RetryOption = new(RetryOption)
				*so.RetryOption = DefaultRetryOption
			}
			for _, opt := range opts {
				if opt != nil {
					opt(so.RetryOption)
				}
			}
		})
	}
	return as
}

// AddToWorkflow implements Builder
func (as AddSteps) AddToWorkflow() map[Steper]*StepConfig { return as }

// Merge another AddSteps into one.
func (as AddSteps) Merge(others ...AddSteps) AddSteps {
	for _, other := range others {
		for k, v := range other {
			if as[k] == nil {
				as[k] = new(StepConfig)
			}
			as[k].Merge(v)
		}
	}
	return as
}

// DependsOn declares dependency on the given Steps.
//
//	Step(a).DependsOn(b, c)
//
// Then b, c should happen-before a.
func (as AddStep[S]) DependsOn(ups ...Steper) AddStep[S] {
	as.AddSteps = as.AddSteps.DependsOn(ups...)
	return as
}

// BeforeStep adds BeforeStep callback for the Step(s).
//
// The BeforeStep callback will be called before Do, and return when first error occurs.
// The order of execution will respect the order of declarations.
// The BeforeStep callbacks are able to change the context.Context feed into Do.
// The BeforeStep callbacks are executed at runtime and per-try.
func (as AddStep[S]) BeforeStep(befores ...BeforeStep) AddStep[S] {
	as.AddSteps = as.AddSteps.BeforeStep(befores...)
	return as
}

// AfterStep adds AfterStep callback for the Step(s).
//
// The AfterStep callback will be called after Do, and pass the error to next AfterStep callback.
// The order of execution will respect the order of declarations.
// The AfterStep callbacks are able to change the error returned by Do.
// The AfterStep callbacks are executed at runtime and per-try.
//
// Tip:
//
// Remember to check error when overriding your own AfterStep function.
//
//	Step(a).AfterStep(func(ctx context.Context, step Steper, err error) error {
//		if err != nil {
//			// do something when error happens
//		}
//		// do something when success
//		return err // you can decide whether pass through the error or not
//	})
func (as AddStep[S]) AfterStep(afters ...AfterStep) AddStep[S] {
	as.AddSteps = as.AddSteps.AfterStep(afters...)
	return as
}

// Timeout sets the Step level timeout.
// Last one wins.
func (as AddStep[S]) Timeout(timeout time.Duration) AddStep[S] {
	as.AddSteps = as.AddSteps.Timeout(timeout)
	return as
}

// When set the Condition for the Step.
// Last one wins.
//
// Tip:
//
// Remember to check upstreams when overriding your own Condition function.
//
//	Steps(a).When(func(ctx context.Context, ups map[Steper]StepResult) StepStatus {
//		// check upstreams leveraging built-in Conditions
//		status := flow.AllSucceeded(ctx, ups)
//		if status != flow.Running {
//			return status // return fast if the Condition not satisfied
//		}
//		// your custom logic
//		if ... {
//			return flow.Running
//		}
//		return flow.Skipped
//	})
func (as AddStep[S]) When(when Condition) AddStep[S] {
	as.AddSteps = as.AddSteps.When(when)
	return as
}

// Retry customize how the Step should be retried.
//
// Step will use DefaultRetryOption when this option is configured with nil.
//
//	w.Add(
//		Step(a), // not retry
//		Step(b).Retry(func(opt *RetryOption) { // will retry 3 times
//			opt.MaxAttempts = 3
//		}),
//		Step(c).Retry(nil), // will use DefaultRetryOption!
//	)
func (as AddStep[S]) Retry(fns ...func(*RetryOption)) AddStep[S] {
	as.AddSteps = as.AddSteps.Retry(fns...)
	return as
}

// AddStep is a typed wrapper of AddSteps.
type AddStep[S Steper] struct {
	AddSteps
	Steps []S
}

// AddSteps helps to add Steper(s) into Workflow.
type AddSteps map[Steper]*StepConfig

// ToSteps converts []<Steper implementation> to []Steper.
//
//	steps := []someStepImpl{ ... }
//	flow.Add(
//		Steps(ToSteps(steps)...),
//	)
func ToSteps[S Steper](steps []S) []Steper {
	rv := []Steper{}
	for _, s := range steps {
		rv = append(rv, s)
	}
	return rv
}

// Merge merges another StepConfig into this one.
func (sc *StepConfig) Merge(other *StepConfig) {
	if other == nil {
		return
	}
	if sc.Upstreams == nil {
		sc.Upstreams = make(Set[Steper])
	}
	sc.Upstreams.Union(other.Upstreams)
	sc.Before = append(sc.Before, other.Before...)
	sc.After = append(sc.After, other.After...)
	sc.Option = append(sc.Option, other.Option...)
}

// Set is a simple generic set implementation based on map.
type Set[T comparable] map[T]struct{}

// Has checks whether the set contains the given value.
func (s Set[T]) Has(v T) bool {
	if s == nil {
		return false
	}
	_, ok := s[v]
	return ok
}

// Add adds the given values into the set.
func (s *Set[T]) Add(vs ...T) {
	if *s == nil {
		*s = make(Set[T])
	}
	for _, v := range vs {
		(*s)[v] = struct{}{}
	}
}

// Union adds all values from the given sets into the set.
func (s *Set[T]) Union(sets ...Set[T]) {
	for _, set := range sets {
		s.Add(set.Flatten()...)
	}
}

// Flatten converts the set into a slice of values.
func (s Set[T]) Flatten() []T {
	r := make([]T, 0, len(s))
	for v := range s {
		r = append(r, v)
	}
	return r
}

// Seq returns an iterator over the set.
// The order of values is indeterminate.
func (s Set[T]) Seq() iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range s {
			if !yield(v) {
				return
			}
		}
	}
}

// Keys returns the keys of the map m.
// The keys will be in an indeterminate order.
//
// Deprecated: use maps.Keys() from standard library instead.
func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	return slices.Collect(maps.Keys(m))
}

// Values returns the values of the map m.
// The values will be in an indeterminate order.
//
// Deprecated: use maps.Values() from standard library instead.
func Values[M ~map[K]V, K comparable, V any](m M) []V {
	return slices.Collect(maps.Values(m))
}
