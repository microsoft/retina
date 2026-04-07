// todo: there are more robust retry packages out there, discuss with team
package retry

import (
	"context"
	"fmt"
	"time"
)

// a Retrier can attempt an operation multiple times, based on some thresholds
type Retrier struct {
	Attempts   int
	Delay      time.Duration
	ExpBackoff bool
}

func (r Retrier) Do(ctx context.Context, f func() error) error {
	var err error
	for i := 0; i < r.Attempts; i++ {
		if ctx.Err() != nil {
			return fmt.Errorf("context error: %w", ctx.Err())
		}
		err = f()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("context error: %w", ctx.Err())
		case <-time.After(r.Delay):
		}
		if r.ExpBackoff {
			r.Delay *= 2
		}
	}
	if err != nil {
		return fmt.Errorf("all retries exhausted [%w]", err)
	}
	return nil
}
