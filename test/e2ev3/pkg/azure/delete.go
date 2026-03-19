// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package azure

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// DeleteResourceGroup is a go-workflow step that deletes a resource group
// and all resources within it.
type DeleteResourceGroup struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
}

func (d *DeleteResourceGroup) Do(ctx context.Context) error {
	log.Printf("deleting resource group %q...", d.ResourceGroupName)

	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}

	clientFactory, err := armresources.NewClientFactory(d.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}

	forceDeleteType := "Microsoft.Compute/virtualMachines,Microsoft.Compute/virtualMachineScaleSets"
	_, err = clientFactory.NewResourceGroupsClient().BeginDelete(ctx, d.ResourceGroupName,
		&armresources.ResourceGroupsClientBeginDeleteOptions{
			ForceDeletionTypes: to.Ptr(forceDeleteType),
		})
	if err != nil {
		return fmt.Errorf("failed to delete resource group %q: %w", d.ResourceGroupName, err)
	}

	log.Printf("resource group %q deleted successfully", d.ResourceGroupName)
	return nil
}

// DeleteCluster is a go-workflow step that deletes an AKS cluster.
type DeleteCluster struct {
	ClusterName       string
	SubscriptionID    string
	ResourceGroupName string
	Location          string
}

func (d *DeleteCluster) Do(ctx context.Context) error {
	log.Printf("deleting cluster %q in resource group %q...", d.ClusterName, d.ResourceGroupName)

	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}

	clientFactory, err := armcontainerservice.NewClientFactory(d.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	poller, err := clientFactory.NewManagedClustersClient().BeginDelete(ctx, d.ResourceGroupName, d.ClusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin cluster deletion: %w", err)
	}

	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to delete cluster %q: %w", d.ClusterName, err)
	}

	log.Printf("cluster %q deleted successfully", d.ClusterName)
	return nil
}
