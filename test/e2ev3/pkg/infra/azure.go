package infra

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/azure"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/azure/arm"
)

// ResolveInfraConfig builds the Azure infrastructure config from viper-loaded values,
// falling back to a random location and generated cluster name when not set.
func ResolveInfraConfig(t *testing.T, ac *azure.Cluster) *azure.InfraConfig {
	t.Helper()

	subID := ac.SubscriptionID
	if subID == "" {
		t.Fatal("AZURE_SUBSCRIPTION_ID must be set")
	}

	location := ac.Location
	if location == "" {
		location = azure.RandomLocation(t)
	}

	clusterName := azure.ClusterNameForE2ETest(t, ac.Name)

	rg := ac.ResourceGroup
	if rg == "" {
		rg = clusterName
	}

	return azure.DefaultE2EInfraConfig(subID, rg, location, clusterName)
}

// AzureSteps returns the workflow steps to deploy Azure infrastructure and
// retrieve the cluster kubeconfig, plus registers teardown via t.Cleanup.
func AzureSteps(t *testing.T, cfg *azure.InfraConfig, kubeConfigFilePath string, createInfra, deleteInfra bool) []flow.Steper {
	var steps []flow.Steper

	if createInfra {
		steps = append(steps, &arm.DeployInfra{Config: cfg})
	}

	steps = append(steps, &azure.GetAKSKubeConfig{
		SubscriptionID:     cfg.SubscriptionID,
		ResourceGroupName:  cfg.ResourceGroupName,
		ClusterName:        cfg.ClusterName,
		Location:           cfg.Location,
		KubeConfigFilePath: kubeConfigFilePath,
	})

	if deleteInfra {
		t.Cleanup(func() {
			del := &arm.DeleteInfra{Config: cfg}
			if err := del.Do(context.Background()); err != nil {
				t.Logf("Failed to delete test infrastructure: %v", err)
			}
		})
	}

	return steps
}
