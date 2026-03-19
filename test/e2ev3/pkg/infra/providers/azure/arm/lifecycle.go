// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package arm

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/azure"
)

// DeleteInfra is a go-workflow step that deletes the resource group created
// by DeployInfra, cascading deletion of all resources within it.
type DeleteInfra struct {
	Config *azure.InfraConfig
}

func (d *DeleteInfra) Do(ctx context.Context) error {
	log.Printf("deleting resource group %q (and all resources within)...", d.Config.ResourceGroupName)

	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain Azure CLI credential: %w", err)
	}

	clientFactory, err := armresources.NewClientFactory(d.Config.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}

	forceDeleteType := "Microsoft.Compute/virtualMachines,Microsoft.Compute/virtualMachineScaleSets"
	poller, err := clientFactory.NewResourceGroupsClient().BeginDelete(ctx, d.Config.ResourceGroupName,
		&armresources.ResourceGroupsClientBeginDeleteOptions{
			ForceDeletionTypes: &forceDeleteType,
		})
	if err != nil {
		return fmt.Errorf("failed to begin resource group deletion: %w", err)
	}

	notifychan := make(chan struct{})
	go func() {
		_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
			Frequency: deploymentPollFrequency,
		})
		close(notifychan)
	}()

	ticker := time.NewTicker(deploymentStatusTicker)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("resource group deletion timed out: %w", ctx.Err())
		case <-ticker.C:
			log.Printf("waiting for resource group %q deletion...", d.Config.ResourceGroupName)
		case <-notifychan:
			if err != nil {
				return fmt.Errorf("resource group %q deletion failed: %w", d.Config.ResourceGroupName, err)
			}
			log.Printf("resource group %q deleted successfully", d.Config.ResourceGroupName)
			return nil
		}
	}
}

// GetKubeConfig is a go-workflow step that retrieves kubeconfig for a cluster
// deployed via ARM template.
type GetKubeConfig struct {
	Config             *azure.InfraConfig
	KubeConfigFilePath string
}

func (g *GetKubeConfig) Do(ctx context.Context) error {
	step := &azure.GetAKSKubeConfig{
		ClusterName:        g.Config.ClusterName,
		SubscriptionID:     g.Config.SubscriptionID,
		ResourceGroupName:  g.Config.ResourceGroupName,
		Location:           g.Config.Location,
		KubeConfigFilePath: g.KubeConfigFilePath,
	}
	return step.Do(ctx)
}
