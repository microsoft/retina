package types

import (
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

func (r *Runner) Run() {
	if r.t.Failed() {
		return
	}
	require.NoError(r.t, r.Job.Run())
}
