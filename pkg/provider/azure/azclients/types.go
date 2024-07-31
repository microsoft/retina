// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package azclients

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"sigs.k8s.io/cloud-provider-azure/pkg/azclient"
)

// AzureConfig represents the configuration for Azure clients.
type AzureConfig struct {
	azclient.AzureAuthConfig `json:",inline" yaml:",inline"`
	azclient.ARMClientConfig `json:",inline" yaml:",inline"`

	SubscriptionID string `json:"subscriptionID,omitempty"`
	// The name of the resource group that the cluster is deployed in
	ResourceGroup string `json:"resourceGroup,omitempty" yaml:"resourceGroup,omitempty"`
	// The location of the resource group that the cluster is deployed in
	Location string `json:"location,omitempty" yaml:"location,omitempty"`
}

type AZClients interface {
	GetClientConfig() *AzureConfig

	GetStorageAccountsClient() *armstorage.AccountsClient
	GetBlobContainersClient() *armstorage.BlobContainersClient
	GetManagementPoliciesClient() *armstorage.ManagementPoliciesClient

	// CreateBlobServiceClient creates a storage account service client on demand.
	CreateBlobServiceClient(storageAccountName string) (*service.Client, error)
	GetBlobServiceClient() *service.Client
}
