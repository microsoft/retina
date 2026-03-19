package flow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/benbjohnson/clock"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockStep struct {
	mock.Mock
	Started chan struct{}
}

func (m *MockStep) Do(ctx context.Context) error {
	var (
		done = make(chan struct{})
	)
	var args mock.Arguments
	go func() {
		defer close(done)
		m.Started <- struct{}{}
		args = m.MethodCalled("Do", ctx)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return args.Error(0)
	}
}

func TestRetry(t *testing.T) {
	type Mock struct {
		w     *flow.Workflow
		clock *clock.Mock
		*MockStep
	}
	newMock := func() *Mock {
		var (
			mockClock = clock.NewMock()
			w         = &flow.Workflow{Clock: mockClock}
			mockStep  = &MockStep{Started: make(chan struct{})}
		)
		w.Add(flow.Step(mockStep).Retry(func(ro *flow.RetryOption) {
			ro.Timer = newTestTimer()
		}))
		return &Mock{
			w:        w,
			clock:    mockClock,
			MockStep: mockStep,
		}
	}
	start := func(m *Mock) <-chan error {
		done := make(chan error)
		go func() {
			var errW flow.ErrWorkflow
			err := m.w.Do(context.Background())
			switch {
			case err == nil:
				done <- nil
			case errors.As(err, &errW):
				done <- errW[m.MockStep]
			}
		}()
		return done
	}
	t.Run("TimeoutPerTry", func(t *testing.T) {
		t.Parallel()
		m := newMock()
		defer m.AssertExpectations(t)
		m.w.Add(
			flow.Step(m.MockStep).Retry(func(ro *flow.RetryOption) {
				ro.TimeoutPerTry = time.Second
				ro.Attempts = 1
			}),
		)
		m.On("Do", mock.Anything).
			Return(nil).
			WaitUntil(m.clock.After(2 * time.Second))
		done := start(m)
		<-m.Started
		m.clock.Add(time.Second)
		assert.ErrorIs(t, <-done, context.DeadlineExceeded)
	})
	t.Run("Attempts", func(t *testing.T) {
		t.Parallel()
		m := newMock()
		defer m.AssertExpectations(t)
		m.w.Add(
			flow.Step(m.MockStep).Retry(func(ro *flow.RetryOption) {
				ro.Attempts = 3
			}),
		)
		var (
			failTwice = m.On("Do", mock.Anything).Return(assert.AnError).Times(2)
			_         = m.On("Do", mock.Anything).Return(nil).NotBefore(failTwice)
		)
		done := start(m)
		<-m.Started
		<-m.Started
		<-m.Started
		assert.NoError(t, <-done)
	})
	t.Run("ShouldRetry", func(t *testing.T) {
		t.Parallel()
		m := newMock()
		defer m.AssertExpectations(t)
		m.w.Add(
			flow.Step(m.MockStep).Retry(func(ro *flow.RetryOption) {
				ro.NextBackOff = func(ctx context.Context, re flow.RetryEvent, nextBackOff time.Duration) time.Duration {
					if re.Attempt > 1 {
						return backoff.Stop
					}
					return nextBackOff
				}
			}),
		)
		m.On("Do", mock.Anything).Return(assert.AnError).Times(3)
		done := start(m)
		<-m.Started
		<-m.Started
		<-m.Started
		assert.ErrorIs(t, <-done, assert.AnError)
	})
	t.Run("Step Level Timeout", func(t *testing.T) {
		t.Parallel()
		m := newMock()
		defer m.AssertExpectations(t)
		m.w.Add(
			flow.Step(m.MockStep).
				Retry(func(ro *flow.RetryOption) {
					ro.TimeoutPerTry = 2 * time.Minute
					ro.Attempts = 2
				}).
				Timeout(time.Minute),
		)
		m.On("Do", mock.Anything).Return(nil).WaitUntil(m.clock.After(time.Hour))
		done := start(m)
		<-m.Started
		m.clock.Add(time.Minute)
		assert.ErrorIs(t, <-done, context.DeadlineExceeded)
	})
}

func newTestTimer() *testTimer {
	return &testTimer{time.NewTimer(0)}
}

// testTimer is a Timer that all retry intervals are immediate (0).
type testTimer struct {
	timer *time.Timer
}

func (t *testTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *testTimer) Start(duration time.Duration) {
	t.timer.Reset(duration)
}

func (t *testTimer) Stop() {
	t.timer.Stop()
}
