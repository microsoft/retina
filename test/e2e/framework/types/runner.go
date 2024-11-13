package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// A wrapper around a job, so that internal job components don't require things like *testing.T
// and can be reused elsewhere
type Runner struct {
	t   *testing.T
	Job *Job
}

func NewRunner(t *testing.T, job *Job) *Runner {
	return &Runner{
		t:   t,
		Job: job,
	}
}

func (r *Runner) Run(ctx context.Context) {
	if r.t.Failed() {
		return
	}
	runComplete := make(chan error)

	go func() {
		runComplete <- r.Job.Run()
		close(runComplete)
	}()
	select {
	case <-ctx.Done():
		r.t.Fatal("Test deadline exceeded. If more time is needed, set -timeout flag to a higher value")
	case err := <-runComplete:
		require.NoError(r.t, err)
	}
}
