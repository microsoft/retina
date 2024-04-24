//go:build e2eframework

package azure

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type CreateResourceGroup struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
}

func (c *CreateResourceGroup) Run() error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx := context.Background()
	clientFactory, err := armresources.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}
	log.Printf("creating resource group %s in location %s...", c.ResourceGroupName, c.Location)

	_, err = clientFactory.NewResourceGroupsClient().CreateOrUpdate(ctx, c.ResourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(c.Location),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the request: %w", err)
	}

	log.Printf("resource group created %s in location %s", c.ResourceGroupName, c.Location)
	return nil
}

func (c *CreateResourceGroup) Prevalidate() error {
	return nil
}

func (c *CreateResourceGroup) Stop() error {
	return nil
}
