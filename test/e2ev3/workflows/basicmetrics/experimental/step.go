//go:build e2e

package experimental

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
)

// Workflow runs the experimental basic metrics workflow.
type Workflow struct {
	Params *config.E2EParams
}

func (s *Workflow) String() string { return "basic-metrics-experimental" }

func (s *Workflow) Do(ctx context.Context) error {
	p := s.Params
	return InstallAndTestRetinaBasicMetricsExperimental(
		p.Paths.KubeConfig, p.Paths.RetinaChart, config.TestPodNamespace,
		&p.Cfg.Image, &p.Cfg.Helm, p.Loader,
	).Do(ctx)
}
