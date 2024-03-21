package azure

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
)

type DeleteCluster struct {
	ClusterName       string
	SubscriptionID    string
	ResourceGroupName string
	Location          string
}

func (d *DeleteCluster) Run() error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx := context.Background()
	clientFactory, err := armcontainerservice.NewClientFactory(d.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	log.Printf("deleting cluster %s in resource group %s...", d.ClusterName, d.ResourceGroupName)
	poller, err := clientFactory.NewManagedClustersClient().BeginDelete(ctx, d.ResourceGroupName, d.ClusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the request: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to pull the result: %w", err)
	}
	return nil
}

func (d *DeleteCluster) Prevalidate() error {
	return nil
}

func (d *DeleteCluster) Stop() error {
	return nil
}
