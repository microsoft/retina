package helpers

import (
	"context"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

const safetyTimeout = 24 * time.Hour

// Context returns a context with a deadline set to the test deadline - 1 min to ensure cleanup.
// If the test deadline is not set, a deadline is set to Now + 24h to prevent the test from running indefinitely
func Context(t *testing.T) (context.Context, context.CancelFunc) {
	deadline, ok := t.Deadline()
	if !ok {
		t.Log("Test deadline disabled, deadline set to Now + 24h to prevent test from running indefinitely")
		deadline = time.Now().Add(safetyTimeout)
	}

	// Subtract a minute from the deadline to ensure we have time to cleanup
	deadline = deadline.Add(-time.Minute)

	ctx, cancel := context.WithDeadline(context.Background(), deadline) //nolint:all // cancel is reassigned in next line
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	return ctx, cancel
}
