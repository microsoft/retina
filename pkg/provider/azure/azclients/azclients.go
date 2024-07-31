// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package azclients

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	storageservice "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"sigs.k8s.io/cloud-provider-azure/pkg/azclient"
	"sigs.k8s.io/cloud-provider-azure/pkg/azclient/configloader"
)

type Impl struct {
	azureConfig *AzureConfig

	storageAccountsClient    *armstorage.AccountsClient
	blobContainersClient     *armstorage.BlobContainersClient
	managementPoliciesClient *armstorage.ManagementPoliciesClient
	blobServiceClient        *storageservice.Client
}

func (azclients *Impl) getTokenCredential() (azcore.TokenCredential, error) {
	// Create token credential out of cloud credential config.
	authProvider, err := azclient.NewAuthProvider(&azclients.azureConfig.ARMClientConfig, &azclients.azureConfig.AzureAuthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AuthProvider, %w", err)
	}
	cred := authProvider.GetAzIdentity()
	return cred, nil
}

func NewAZClients(configFile string) (AZClients, error) {
	azclients := Impl{}
	// Load auth config file from cloud credential file.
	config, err := configloader.Load[AzureConfig](context.Background(),
		nil,
		&configloader.FileLoaderConfig{FilePath: configFile},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load azure credential config, %w", err)
	}
	azclients.azureConfig = config

	cred, err := azclients.getTokenCredential()
	if err != nil {
		return nil, fmt.Errorf("failed to get TokenCredential, %w", err)
	}

	// Create default necessary az clients.

	storageClientFactory, err := armstorage.NewClientFactory(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClientFactory, %w", err)
	}
	azclients.storageAccountsClient = storageClientFactory.NewAccountsClient()
	azclients.blobContainersClient = storageClientFactory.NewBlobContainersClient()
	azclients.managementPoliciesClient = storageClientFactory.NewManagementPoliciesClient()

	return &azclients, nil
}

func (azclients *Impl) GetStorageAccountsClient() *armstorage.AccountsClient {
	return azclients.storageAccountsClient
}

func (azclients *Impl) GetBlobContainersClient() *armstorage.BlobContainersClient {
	return azclients.blobContainersClient
}

func (azclients *Impl) GetManagementPoliciesClient() *armstorage.ManagementPoliciesClient {
	return azclients.managementPoliciesClient
}

func (azclients *Impl) CreateBlobServiceClient(storageAccountName string) (*storageservice.Client, error) {
	cred, err := azclients.getTokenCredential()
	if err != nil {
		return nil, fmt.Errorf("failed to get TokenCredential, %w", err)
	}
	azclients.blobServiceClient, err = storageservice.NewClient(fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccountName), cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob service client, %w", err)
	}

	return azclients.blobServiceClient, nil
}

func (azclients *Impl) GetBlobServiceClient() *storageservice.Client {
	return azclients.blobServiceClient
}

func (azclients *Impl) GetClientConfig() *AzureConfig {
	return azclients.azureConfig
}
