// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package utils

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
)

const (
	// DefaultValidationTimeout bounds total time for validation within a port-forward.
	DefaultValidationTimeout = 5 * time.Minute

	// DefaultRetryAttempts for metric validation (metrics may need time to appear).
	DefaultRetryAttempts = 10

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

func (w *WithPortForward) String() string { return "with-port-forward" }

func (w *WithPortForward) Do(ctx context.Context) error {
	log := slog.With("step", w.String())
	if err := w.PF.Do(ctx); err != nil {
		return fmt.Errorf("port-forward failed: %w", err)
	}
	defer func() {
		log.Info("stopping port-forward", "local", w.PF.LocalPort, "remote", w.PF.RemotePort)
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
			slog.Info("curl failed as expected", "step", name, "error", err)
		}
		return nil
	})
}

// RetryWithBackoff configures exponential backoff for metric validation.
func RetryWithBackoff(ro *flow.RetryOption) {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 5 * time.Second
	bo.MaxInterval = 30 * time.Second
	bo.MaxElapsedTime = 5 * time.Minute
	ro.Backoff = bo
	ro.Attempts = DefaultRetryAttempts
	ro.TimeoutPerTry = 30 * time.Second
}
