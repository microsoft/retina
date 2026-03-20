package infra

import (
	"context"
	"fmt"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/azure"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/kind"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Workflow provisions a cluster via the configured provider.
type Workflow struct {
	Cfg *config.E2EConfig
	T   *testing.T
}

func (s *Workflow) String() string { return "setup-infra" }

func (s *Workflow) Do(ctx context.Context) error {
	ctx = utils.WithWorkflow(ctx, s.String())
	p := s.Cfg
	kubeCfgPath := p.Cluster.KubeConfigPath()

	if *config.KubeConfig != "" {
		rc, err := clientcmd.BuildConfigFromFlags("", kubeCfgPath)
		if err != nil {
			return fmt.Errorf("build rest config: %w", err)
		}
		setRestConfig(p.Cluster, rc)
		return nil
	}

	var steps []flow.Steper
	switch *config.Provider {
	case "kind":
		kc := p.Cluster.(*kind.Cluster)
		kindCfg := kind.DefaultE2EKindConfig(kc.Name)
		kc.Name = kindCfg.ClusterName
		steps = KindSteps(s.T, kindCfg, kubeCfgPath, *config.CreateInfra, *config.DeleteInfra)
	default:
		ac := p.Cluster.(*azure.Cluster)
		infraCfg := ResolveInfraConfig(s.T, ac)
		steps = AzureSteps(s.T, infraCfg, kubeCfgPath, *config.CreateInfra, *config.DeleteInfra)
	}

	inner := new(flow.Workflow)
	inner.Add(flow.Pipe(steps...))
	if err := inner.Do(ctx); err != nil {
		return err
	}

	rc, err := clientcmd.BuildConfigFromFlags("", kubeCfgPath)
	if err != nil {
		return fmt.Errorf("build rest config: %w", err)
	}
	setRestConfig(p.Cluster, rc)
	return nil
}

func setRestConfig(c config.ClusterProvider, rc *rest.Config) {
	switch t := c.(type) {
	case *kind.Cluster:
		t.RC = rc
	case *azure.Cluster:
		t.RC = rc
	}
}
