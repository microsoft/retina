//go:build e2e

package hubblemetrics

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
)

// Workflow runs the hubble metrics workflow.
type Workflow struct {
	Params *config.E2EParams
}

func (s *Workflow) String() string { return "hubble-metrics" }

func (s *Workflow) Do(ctx context.Context) error {
	p := s.Params
	return InstallAndTestHubbleMetrics(
		p.Paths.KubeConfig, p.Paths.HubbleChart,
		&p.Cfg.Image, &p.Cfg.Helm, p.Loader,
	).Do(ctx)
}
