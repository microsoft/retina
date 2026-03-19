// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"context"
	"fmt"
	"log"
	"time"

	flow "github.com/Azure/go-workflow"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
)

const (
	// DefaultValidationTimeout bounds total time for validation within a port-forward.
	DefaultValidationTimeout = 5 * time.Minute

	// DefaultRetryAttempts for metric validation (metrics may need time to appear).
	DefaultRetryAttempts = 5

	// DefaultScenarioTimeout bounds the total setup phase of a scenario.
	DefaultScenarioTimeout = 10 * time.Minute
)

// WithPortForward is a composite step that:
//  1. Starts a Kubernetes port-forward
//  2. Runs all inner steps sequentially (as a Pipe)
//  3. Guarantees the port-forward is stopped via defer, even on error
type WithPortForward struct {
	PF    *k8s.PortForward
	Steps []flow.Steper
}

func (w *WithPortForward) Do(ctx context.Context) error {
	if err := w.PF.Do(ctx); err != nil {
		return fmt.Errorf("port-forward failed: %w", err)
	}
	defer func() {
		log.Printf("stopping port-forward %s → %s", w.PF.LocalPort, w.PF.RemotePort)
		w.PF.Stop() //nolint:errcheck // best-effort cleanup
	}()

	inner := new(flow.Workflow)
	inner.Add(flow.Pipe(w.Steps...))
	if err := inner.Do(ctx); err != nil {
		return fmt.Errorf("validation within port-forward failed: %w", err)
	}
	return nil
}

// Unwrap exposes inner steps to go-workflow for visibility/debugging.
func (w *WithPortForward) Unwrap() []flow.Steper {
	return w.Steps
}

// CurlExpectFail creates a named step that runs a command expected to fail
// (e.g., curl behind a deny-all network policy). The error is intentionally swallowed.
func CurlExpectFail(name string, exec *k8s.ExecInPod) flow.Steper {
	return flow.Func(name, func(ctx context.Context) error {
		if err := exec.Do(ctx); err != nil {
			log.Printf("[%s] curl failed as expected: %v", name, err)
		}
		return nil
	})
}

// AddScenario wires the standard scenario pattern into a workflow:
//
//	setup steps → validate (with retry) → cleanup (always)
//
// The setup steps run sequentially after dependsOn, bounded by a timeout.
// The validation step retries with exponential backoff.
// Cleanup steps run regardless of validation outcome.
// Returns the last cleanup step (or validateWithPF if no cleanup).
func AddScenario(
	wf *flow.Workflow,
	dependsOn flow.Steper,
	setup []flow.Steper,
	validateWithPF *WithPortForward,
	cleanup []flow.Steper,
) flow.Steper {
	// Setup: sequential chain, bounded by timeout.
	lastSetup := dependsOn
	if len(setup) > 0 {
		wf.Add(
			flow.Pipe(setup...).
				DependsOn(dependsOn).
				Timeout(DefaultScenarioTimeout),
		)
		lastSetup = setup[len(setup)-1]
	}

	// Validate: retry with exponential backoff.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(lastSetup).
			Retry(retryWithBackoff),
	)

	// Cleanup: runs even if validation fails.
	if len(cleanup) > 0 {
		wf.Add(
			flow.Pipe(cleanup...).
				DependsOn(validateWithPF).
				When(flow.Always),
		)
		return cleanup[len(cleanup)-1]
	}
	return validateWithPF
}

// retryWithBackoff configures exponential backoff for metric validation.
func retryWithBackoff(ro *flow.RetryOption) {
	ro.Attempts = DefaultRetryAttempts
	ro.TimeoutPerTry = 30 * time.Second
	// Backoff is exponential by default (flow.DefaultRetryOption uses exponential backoff).
}
