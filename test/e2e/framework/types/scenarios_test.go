package types

import (
	"fmt"
	"testing"
)

// Test against a BYO cluster with Cilium and Hubble enabled,
// create a pod with a deny all network policy and validate
// that the drop metrics are present in the prometheus endpoint
func TestScenarioValues(t *testing.T) {
	job := NewJob("Validate that drop metrics are present in the prometheus endpoint")
	runner := NewRunner(t, job)
	defer runner.Run()

	// Add top level step
	job.AddStep(&DummyStep{
		Parameter1: "Top Level Step 1",
		Parameter2: "Top Level Step 2",
	}, nil)

	// Add scenario to ensure that the parameters are set correctly
	// and inherited without overriding
	job.AddScenario(NewDummyScenario())

	job.AddStep(&DummyStep{}, nil)
}

// Test against a BYO cluster with Cilium and Hubble enabled,
// create a pod with a deny all network policy and validate
// that the drop metrics are present in the prometheus endpoint
func TestScenarioValuesWithSkip(t *testing.T) {
	job := NewJob("Validate that drop metrics are present in the prometheus endpoint")
	runner := NewRunner(t, job)
	defer runner.Run()

	// Add top level step
	job.AddStep(&DummyStep{
		Parameter1: "Top Level Step 1",
		Parameter2: "Top Level Step 2",
	}, &StepOptions{
		SkipSavingParamatersToJob: true,
	})

	// top level step skips saving parameters, so we should error here
	// that parameters are missing
	job.AddScenario(NewDummyScenario())

	job.AddStep(&DummyStep{
		Parameter1: "Other Level Step 1",
		Parameter2: "Other Level Step 2",
	}, nil)
}

func TestScenarioValuesWithScenarioSkip(t *testing.T) {
	job := NewJob("Validate that drop metrics are present in the prometheus endpoint")
	runner := NewRunner(t, job)
	defer runner.Run()

	// Add top level step
	job.AddStep(&DummyStep{
		Parameter1: "Kubeconfig path 1",
		Parameter2: "Kubeconfig path 2",
	}, nil)

	// top level step skips saving parameters, so we should error here
	// that parameters are missing
	job.AddScenario(NewDummyScenarioWithSkipSave())

	// Add top level step
	job.AddStep(&DummyStep{}, nil)
}

func NewDummyScenario() *Scenario {
	return NewScenario("Dummy Scenario",
		&StepWrapper{
			Step: &DummyStep{
				Parameter1: "Something in Scenario 1",
				Parameter2: "Something in Scenario 1",
			},
		},
	)
}

func NewDummyScenario2() *Scenario {
	return NewScenario("Dummy Scenario",
		&StepWrapper{
			Step: &DummyStep{
				Parameter1: "Something 2 in Scenario 1",
				Parameter2: "Something 2 in Scenario 1",
			},
		},
	)
}

func NewDummyScenarioWithSkipSave() *Scenario {
	return NewScenario("Dummy Scenario",
		&StepWrapper{
			Step: &DummyStep{
				Parameter1: "",
				Parameter2: "",
			}, Opts: &StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
	)
}

type DummyStep struct {
	Parameter1 string
	Parameter2 string
}

func (d *DummyStep) Run() error {
	fmt.Printf("Running DummyStep with parameter 1 as: %s\n", d.Parameter1)
	fmt.Printf("Running DummyStep with parameter 2 as: %s\n", d.Parameter2)
	return nil
}

func (d *DummyStep) Stop() error {
	return nil
}

func (d *DummyStep) Prevalidate() error {
	return nil
}
