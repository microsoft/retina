package types

import (
	"errors"
	"testing"
)

var ErrKeyAlreadyExists = errors.New("key already exists in runtime objects")

type testobj int64

// Test against a BYO cluster with Cilium and Hubble enabled,
// create a pod with a deny all network policy and validate
// that the drop metrics are present in the prometheus endpoint
func TestRuntimeObjects(t *testing.T) {
	job := NewJob("Test Runtime Objects injection behavior")
	runner := NewRunner(t, job)
	defer runner.Run()

	job.AddStep(&RuntimeObjectsTestStep{
		TestIntKey: "test integer",
	}, &StepOptions{
		ExpectError:               false,
		SkipSavingParametersToJob: true,
	})

	job.AddStep(&RuntimeObjectsTestStep{
		TestIntKey: "test integer",
	}, &StepOptions{
		ExpectError:               true,
		SkipSavingParametersToJob: true,
	})
}

type RuntimeObjectsTestStep struct {
	TestIntKey string
}

func (d *RuntimeObjectsTestStep) Run(ro *RuntimeObjects) error {
	if retrieved, ok := ro.Get(d.TestIntKey); ok {
		if retrieved.(testobj) != 1 {
			return errors.New("value retrieved from runtime objects does not match expected value") //nolint This is a test
		}

		return ErrKeyAlreadyExists
	}

	testintvalue := testobj(1)

	_, err := ro.SetGet(d.TestIntKey, testintvalue)
	if err != nil {
		return err
	}

	return nil
}

func (d *RuntimeObjectsTestStep) Stop() error {
	return nil
}

func (d *RuntimeObjectsTestStep) PreRun() error {
	return nil
}
