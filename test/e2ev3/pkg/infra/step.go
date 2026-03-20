package infra

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/kind"
)

// Workflow provisions a cluster via the configured provider.
type Workflow struct {
	Params *config.E2EParams
	T      *testing.T
}

func (s *Workflow) String() string { return "setup-infra" }

func (s *Workflow) Do(ctx context.Context) error {
	p := s.Params
	if *config.KubeConfig != "" {
		return nil
	}

	var steps []flow.Steper
	switch *config.Provider {
	case "kind":
		kindCfg := kind.DefaultE2EKindConfig(p.Cfg.Azure.ClusterName)
		steps = KindSteps(s.T, kindCfg, p.Paths.KubeConfig, *config.CreateInfra, *config.DeleteInfra)
	default:
		infraCfg := ResolveInfraConfig(s.T, &p.Cfg.Azure)
		steps = AzureSteps(s.T, infraCfg, p.Paths.KubeConfig, *config.CreateInfra, *config.DeleteInfra)
	}

	inner := new(flow.Workflow)
	inner.Add(flow.Pipe(steps...))
	return inner.Do(ctx)
}
