package types

var DefaultOpts = StepOptions{
	// when wanting to expect an error, set to true
	ExpectError: false,

	// when wanting to avoid saving the parameters to the job,
	// such as a repetetive task where step is used multiple times sequentially,
	// but parameters are different each time
	SkipSavingParametersToJob: false,
}

type Step interface {
	// Useful when wanting to do parameter checking, for example
	// if a parameter length is known to be required less than 80 characters,
	// do this here so we don't find out later on when we run the step
	// when possible, try to avoid making external calls, this should be fast and simple
	Prevalidate() error

	// Primary step where test logic is executed
	// Returning an error will cause the test to fail
	Run() error

	// Require for background steps
	Stop() error
}

type StepOptions struct {
	ExpectError bool

	// Generally set this to false when you want to reuse
	// a step, but you don't want to save the parameters
	// ex: Sleep for 15 seconds, then Sleep for 10 seconds,
	// you don't want to save the parameters
	SkipSavingParametersToJob bool

	// Will save this step to the job's steps
	// and then later on when Stop is called with job name,
	// it will call Stop() on the step
	RunInBackgroundWithID string
}
