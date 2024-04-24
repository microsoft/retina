//go:build e2eframework

package azure

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
)

const KubeConfigPerms = 0o600

type GetAKSKubeConfig struct {
	ClusterName        string
	SubscriptionID     string
	ResourceGroupName  string
	Location           string
	KubeConfigFilePath string
}

func (c *GetAKSKubeConfig) Run() error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx := context.Background()
	clientFactory, err := armcontainerservice.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	res, err := clientFactory.NewManagedClustersClient().ListClusterUserCredentials(ctx, c.ResourceGroupName, c.ClusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the get managed cluster client request: %w", err)
	}

	err = os.WriteFile(c.KubeConfigFilePath, []byte(res.Kubeconfigs[0].Value), KubeConfigPerms)
	if err != nil {
		return fmt.Errorf("failed to write kubeconfig to file \"%s\": %w", c.KubeConfigFilePath, err)
	}

	log.Printf("kubeconfig for cluster \"%s\" in resource group \"%s\" written to \"%s\"\n", c.ClusterName, c.ResourceGroupName, c.KubeConfigFilePath)
	return nil
}

func (c *GetAKSKubeConfig) Prevalidate() error {
	return nil
}

func (c *GetAKSKubeConfig) Stop() error {
	return nil
}
