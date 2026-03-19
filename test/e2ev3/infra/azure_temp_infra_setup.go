package infra

import (
	"context"
	"testing"

	"github.com/microsoft/retina/test/e2ev3/pkg/azure"
	"github.com/microsoft/retina/test/e2ev3/pkg/azure/arm"
	"github.com/stretchr/testify/require"
)

// CreateAzureTempK8sInfra provisions (or connects to) Azure infrastructure for e2e testing.
// It deploys all resources in a single ARM template and registers cleanup on test completion.
func CreateAzureTempK8sInfra(ctx context.Context, t *testing.T, cfg *azure.InfraConfig, kubeConfigFilePath string, createInfra, deleteInfra bool) {
	if createInfra {
		deploy := &arm.DeployInfra{Config: cfg}
		require.NoError(t, deploy.Do(ctx), "failed to deploy ARM template")
	}

	getKubeConfig := &azure.GetAKSKubeConfig{
		SubscriptionID:     cfg.SubscriptionID,
		ResourceGroupName:  cfg.ResourceGroupName,
		ClusterName:        cfg.ClusterName,
		Location:           cfg.Location,
		KubeConfigFilePath: kubeConfigFilePath,
	}
	require.NoError(t, getKubeConfig.Do(ctx), "failed to get kubeconfig")

	if deleteInfra {
		t.Cleanup(func() {
			deleteStep := &arm.DeleteInfra{Config: cfg}
			if err := deleteStep.Do(context.Background()); err != nil {
				t.Logf("Failed to delete test infrastructure: %v", err)
			}
		})
	}
}
