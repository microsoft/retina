package legacy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type CreateResourceGroup struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
}

func (c *CreateResourceGroup) Do(_ context.Context) error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx := context.Background()
	clientFactory, err := armresources.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}
	slog.Info("creating resource group", "resourceGroup", c.ResourceGroupName, "location", c.Location)

	_, err = clientFactory.NewResourceGroupsClient().CreateOrUpdate(ctx, c.ResourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(c.Location),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the request: %w", err)
	}

	slog.Info("resource group created", "resourceGroup", c.ResourceGroupName, "location", c.Location)
	return nil
}
