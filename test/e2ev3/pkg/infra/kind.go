package infra

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/kind"
)

// KindSteps returns the workflow steps to provision a Kind cluster and
// export its kubeconfig, plus registers teardown via t.Cleanup.
func KindSteps(t *testing.T, cfg *kind.Config, kubeConfigFilePath string, createInfra, deleteInfra bool) []flow.Steper {
	var steps []flow.Steper

	if createInfra {
		steps = append(steps, &kind.CreateCluster{Config: cfg})
	}

	steps = append(steps, &kind.ExportKubeConfig{
		ClusterName:        cfg.ClusterName,
		KubeConfigFilePath: kubeConfigFilePath,
	})

	if createInfra {
		steps = append(steps, &kind.InstallNPM{
			KubeConfigFilePath: kubeConfigFilePath,
		})
	}

	if deleteInfra {
		t.Cleanup(func() {
			del := &kind.DeleteCluster{
				ClusterName:        cfg.ClusterName,
				KubeConfigFilePath: kubeConfigFilePath,
			}
			if err := del.Do(context.Background()); err != nil {
				t.Logf("Failed to delete Kind cluster: %v", err)
			}
		})
	}

	return steps
}
