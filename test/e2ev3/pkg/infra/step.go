package infra

import (
	"context"
	"fmt"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/kind"
	"k8s.io/client-go/tools/clientcmd"
)

// Workflow provisions a cluster via the configured provider.
type Workflow struct {
	Cfg *config.E2EConfig
	T      *testing.T
}

func (s *Workflow) String() string { return "setup-infra" }

func (s *Workflow) Do(ctx context.Context) error {
	p := s.Cfg
	if *config.KubeConfig != "" {
		rc, err := clientcmd.BuildConfigFromFlags("", p.Paths.KubeConfig)
		if err != nil {
			return fmt.Errorf("build rest config: %w", err)
		}
		p.RestConfig = rc
		return nil
	}

	var steps []flow.Steper
	switch *config.Provider {
	case "kind":
		kindCfg := kind.DefaultE2EKindConfig(p.Azure.ClusterName)
		steps = KindSteps(s.T, kindCfg, p.Paths.KubeConfig, *config.CreateInfra, *config.DeleteInfra)
	default:
		infraCfg := ResolveInfraConfig(s.T, &p.Azure)
		steps = AzureSteps(s.T, infraCfg, p.Paths.KubeConfig, *config.CreateInfra, *config.DeleteInfra)
	}

	inner := new(flow.Workflow)
	inner.Add(flow.Pipe(steps...))
	if err := inner.Do(ctx); err != nil {
		return err
	}

	rc, err := clientcmd.BuildConfigFromFlags("", p.Paths.KubeConfig)
	if err != nil {
		return fmt.Errorf("build rest config: %w", err)
	}
	p.RestConfig = rc
	return nil
}
