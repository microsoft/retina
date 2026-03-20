// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package azure

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
)

const kubeConfigPerms = 0o600

// GetAKSKubeConfig is a go-workflow step that retrieves cluster credentials
// and writes the kubeconfig to a file.
type GetAKSKubeConfig struct {
	ClusterName        string
	SubscriptionID     string
	ResourceGroupName  string
	Location           string
	KubeConfigFilePath string
}

func (c *GetAKSKubeConfig) String() string { return "get-aks-kubeconfig" }

func (c *GetAKSKubeConfig) Do(ctx context.Context) error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}

	clientFactory, err := armcontainerservice.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	res, err := clientFactory.NewManagedClustersClient().ListClusterUserCredentials(ctx, c.ResourceGroupName, c.ClusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster credentials: %w", err)
	}

	if err := os.WriteFile(c.KubeConfigFilePath, res.Kubeconfigs[0].Value, kubeConfigPerms); err != nil {
		return fmt.Errorf("failed to write kubeconfig to %q: %w", c.KubeConfigFilePath, err)
	}

	slog.Info("kubeconfig for cluster written", "cluster", c.ClusterName, "path", c.KubeConfigFilePath)
	return nil
}

// GetFQDN returns the FQDN of the given AKS cluster.
func GetFQDN(ctx context.Context, subscriptionID, resourceGroupName, clusterName string) (string, error) {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return "", fmt.Errorf("failed to obtain a credential: %w", err)
	}

	clientFactory, err := armcontainerservice.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	res, err := clientFactory.NewManagedClustersClient().Get(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster: %w", err)
	}

	return *res.Properties.Fqdn, nil
}
