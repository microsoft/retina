package capture

import (
	"context"
	"os/signal"
	"syscall"
	"testing"
)

func TestContext(t *testing.T) (context.Context, context.CancelFunc) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)

	deadline, ok := t.Deadline()
	if ok {
		return context.WithDeadline(ctx, deadline)
	}

	return ctx, cancel
}
