//go:build e2e

package capture

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
)

// Workflow runs the capture validation workflow.
type Workflow struct {
	Params *config.E2EParams
}

func (s *Workflow) String() string { return "capture" }

func (s *Workflow) Do(ctx context.Context) error {
	p := s.Params
	return ValidateCapture(
		p.Paths.KubeConfig, "default",
		&p.Cfg.Image,
	).Do(ctx)
}
