// todo: there are more robust retry packages out there, discuss with team
package retry

import (
	"context"
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
		return ctx.Err()
	case <-done:
		return err
	}
}
