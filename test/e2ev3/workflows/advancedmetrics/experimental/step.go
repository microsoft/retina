//go:build e2e

package experimental

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
)

// Workflow runs the experimental advanced metrics workflow.
type Workflow struct {
	Params *config.E2EParams
}

func (s *Workflow) String() string { return "advanced-metrics-experimental" }

func (s *Workflow) Do(ctx context.Context) error {
	p := s.Params
	return UpgradeAndTestRetinaAdvancedMetricsExperimental(
		p.Paths.KubeConfig, p.Paths.RetinaChart,
		p.Paths.AdvancedProfile, config.TestPodNamespace,
		&p.Cfg.Helm,
	).Do(ctx)
}
