package flow

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var DefaultRetryOption = RetryOption{
	Backoff:  backoff.NewExponentialBackOff(),
	Attempts: 3,
}

// RetryOption customizes retry behavior of a Step in Workflow.
type RetryOption struct {
	TimeoutPerTry time.Duration // 0 means no timeout
	Attempts      uint64        // 0 means no limit
	// NextBackOff is called after each retry to determine the next backoff duration.
	// Notice if attempts limits are reach, or context timeout, or BackOff fires backoff.Stop,
	// this function will not be called.
	//
	// RetryEvent: the event records attempt, duration since the start, and the error of the last try.
	// nextBackOff: the next backoff duration calculated by the inner BackOff
	NextBackOff func(ctx context.Context, re RetryEvent, nextBackOff time.Duration) time.Duration

	Backoff backoff.BackOff
	Notify  backoff.Notify
	Timer   backoff.Timer
}

// RetryEvent is the event fired when a retry happens
type RetryEvent struct {
	Attempt uint64
	Since   time.Duration
	Error   error
}

// retry constructs a do function with retry enabled according to the option.
func (w *Workflow) retry(opt *RetryOption) func(
	ctx context.Context,
	do func(context.Context) error,
	notAfter time.Time, // the Step level timeout ddl
) error {
	if opt == nil {
		return func(ctx context.Context, do func(context.Context) error, notAfter time.Time) error { return do(ctx) }
	}
	return func(ctx context.Context, do func(context.Context) error, notAfter time.Time) error {
		backOff := opt.Backoff
		backOff = backoff.WithContext(backOff, ctx)
		if !notAfter.IsZero() {
			backOff = &backOffStopIfTimeout{BackOff: backOff, NotAfter: notAfter, Now: w.Clock.Now}
		}
		if opt.Attempts > 0 {
			backOff = backoff.WithMaxRetries(backOff, opt.Attempts-1)
		}
		retried := func(ctx context.Context, e RetryEvent) {}
		if opt.NextBackOff != nil {
			b := &backOffWithEvent{BackOff: backOff, nextBackOff: opt.NextBackOff}
			retried = b.retried
			backOff = b
		}
		e := RetryEvent{Attempt: 0}
		start := w.Clock.Now()
		return backoff.RetryNotifyWithTimer(
			func() error {
				defer func() {
					retried(ctx, e)
					e.Attempt++
				}()
				ctxPerTry := ctx
				if opt.TimeoutPerTry > 0 {
					var cancel context.CancelFunc
					ctxPerTry, cancel = w.Clock.WithTimeout(ctx, opt.TimeoutPerTry)
					defer cancel()
				}
				err := do(ctxPerTry)
				e.Since = w.Clock.Since(start)
				e.Error = err
				return err
			},
			backOff,
			opt.Notify,
			opt.Timer,
		)
	}
}

type backOffWithEvent struct {
	backoff.BackOff
	nextBackOff func(context.Context, RetryEvent, time.Duration) time.Duration

	ctx context.Context
	e   RetryEvent
}

func (b *backOffWithEvent) NextBackOff() time.Duration {
	bkof := b.BackOff.NextBackOff()
	if b.nextBackOff == nil || bkof == backoff.Stop {
		return backoff.Stop
	}
	return b.nextBackOff(b.ctx, b.e, bkof)
}
func (b *backOffWithEvent) retried(ctx context.Context, e RetryEvent) {
	b.ctx = ctx
	b.e = e
}

type backOffStopIfTimeout struct {
	backoff.BackOff
	NotAfter time.Time
	Now      func() time.Time
}

func (b *backOffStopIfTimeout) NextBackOff() time.Duration {
	bkof := b.BackOff.NextBackOff()
	if b.NotAfter.IsZero() || b.Now == nil || bkof == backoff.Stop || b.Now().After(b.NotAfter) {
		return backoff.Stop
	}
	return bkof
}
