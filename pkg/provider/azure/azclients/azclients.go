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

type AZClientsImpl struct {
	azureConfig *AzureConfig

	storageAccountsClient    *armstorage.AccountsClient
	blobContainersClient     *armstorage.BlobContainersClient
	managementPoliciesClient *armstorage.ManagementPoliciesClient
	blobServiceClient        *storageservice.Client
}

func (azclients *AZClientsImpl) getTokenCredential() (azcore.TokenCredential, error) {
	// Create token credential out of cloud credential config.
	authProvider, err := azclient.NewAuthProvider(&azclients.azureConfig.ARMClientConfig, &azclients.azureConfig.AzureAuthConfig)
	if err != nil {
		return nil, err
	}
	cred := authProvider.GetAzIdentity()
	return cred, nil
}

func NewAZClients(configFile string) (AZClients, error) {
	azclients := AZClientsImpl{}
	// Load auth config file from cloud credential file.
	config, err := configloader.Load[AzureConfig](context.Background(),
		nil,
		&configloader.FileLoaderConfig{FilePath: configFile},
	)
	if err != nil {
		return nil, err
	}
	azclients.azureConfig = config

	cred, err := azclients.getTokenCredential()
	if err != nil {
		return nil, err
	}

	// Create default necessary az clients.

	storageClientFactory, err := armstorage.NewClientFactory(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	azclients.storageAccountsClient = storageClientFactory.NewAccountsClient()
	azclients.blobContainersClient = storageClientFactory.NewBlobContainersClient()
	azclients.managementPoliciesClient = storageClientFactory.NewManagementPoliciesClient()

	return &azclients, nil
}

func (azclients *AZClientsImpl) GetStorageAccountsClient() *armstorage.AccountsClient {
	return azclients.storageAccountsClient
}

func (azclients *AZClientsImpl) GetBlobContainersClient() *armstorage.BlobContainersClient {
	return azclients.blobContainersClient
}

func (azclients *AZClientsImpl) GetManagementPoliciesClient() *armstorage.ManagementPoliciesClient {
	return azclients.managementPoliciesClient
}

func (azclients *AZClientsImpl) CreateBlobServiceClient(storageAccountName string) (*storageservice.Client, error) {
	cred, err := azclients.getTokenCredential()
	if err != nil {
		return nil, err
	}
	azclients.blobServiceClient, err = storageservice.NewClient(fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccountName), cred, nil)
	if err != nil {
		return nil, err
	}

	return azclients.blobServiceClient, nil
}

func (azclients *AZClientsImpl) GetBlobServiceClient() *storageservice.Client {
	return azclients.blobServiceClient
}

func (azclients *AZClientsImpl) GetClientConfig() *AzureConfig {
	return azclients.azureConfig
}
