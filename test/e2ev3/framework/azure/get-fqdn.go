package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
)

func GetFqdnFn(subscriptionId, resourceGroupName, clusterName string) (string, error) {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return "", fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx := context.Background()
	clientFactory, err := armcontainerservice.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}
	res, err := clientFactory.NewManagedClustersClient().Get(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to finish the get managed cluster client request: %w", err)
	}

	return *res.Properties.Fqdn, nil
}
