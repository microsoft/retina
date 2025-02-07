package capture

import (
	"context"
	"os/signal"
	"syscall"
	"testing"
)

func createTestContext(t *testing.T) (context.Context, func()) {
	deadline, ok := t.Deadline()
	if !ok {
		return signal.NotifyContext(context.Background(), syscall.SIGTERM)
	} else {
		return context.WithDeadline(context.Background(), deadline)
	}
}
