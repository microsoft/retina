//go:build e2eframework

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
	done := make(chan struct{})
	var err error
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < r.Attempts; i++ {
			err = f()
			if err == nil {
				break
			}
			time.Sleep(r.Delay)
			if r.ExpBackoff {
				r.Delay *= 2
			}
		}
	}()

	select {
	case <-ctx.Done():
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return fmt.Errorf("context error: %w", ctxErr)
		}
		return nil
	case <-done:
		return err
	}
}
