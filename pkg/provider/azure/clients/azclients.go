package clients

import (
	"context"
	"fmt"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	storageservice "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"sigs.k8s.io/cloud-provider-azure/pkg/azclient"
	"sigs.k8s.io/cloud-provider-azure/pkg/azclient/configloader"
)

type AZClients struct {
	AzureConfig *AzureConfig

	StorageAccountsClient    *armstorage.AccountsClient
	BlobContainersClient     *armstorage.BlobContainersClient
	ManagementPoliciesClient *armstorage.ManagementPoliciesClient
	blobServiceClient        *storageservice.Client
}

func (azclients *AZClients) getTokenCredential() (azcore.TokenCredential, error) {
	// Create token credential out of cloud credential config.
	authProvider, err := azclient.NewAuthProvider(&azclients.AzureConfig.ARMClientConfig, &azclients.AzureConfig.AzureAuthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AuthProvider: %w", err)
	}
	cred := authProvider.GetAzIdentity()
	return cred, nil
}

func NewAZClients(configFile string) (*AZClients, error) {
	azclients := &AZClients{}
	// Load auth config file from cloud credential file.
	config, err := configloader.Load[AzureConfig](context.Background(),
		nil,
		&configloader.FileLoaderConfig{FilePath: configFile},
	)
	if err != nil {
		return azclients, fmt.Errorf("failed to load azure credential config: %w", err)
	}
	azclients.AzureConfig = config

	cred, err := azclients.getTokenCredential()
	if err != nil {
		return azclients, fmt.Errorf("failed to get TokenCredential: %w", err)
	}

	// Create default necessary az clients.

	storageClientFactory, err := armstorage.NewClientFactory(config.SubscriptionID, cred, nil)
	if err != nil {
		return azclients, fmt.Errorf("failed to create ClientFactory: %w", err)
	}
	azclients.StorageAccountsClient = storageClientFactory.NewAccountsClient()
	azclients.BlobContainersClient = storageClientFactory.NewBlobContainersClient()
	azclients.ManagementPoliciesClient = storageClientFactory.NewManagementPoliciesClient()

	return azclients, nil
}

// GetBlobServiceClient gives a blob service client for a given storage account.
func (azclients *AZClients) GetBlobServiceClient(storageAccountName string) (*storageservice.Client, error) {
	// validate the storage account to eliminate potential security risks.
	// For example, a malicious user could use a storage account name that contains
	// special characters to perform a path traversal attack or DDOS.
	validName := regexp.MustCompile(`^[a-z0-9]{3,24}$`)
	if match := validName.MatchString(storageAccountName); !match {
		return nil, fmt.Errorf("invalid storage account name: %s", storageAccountName) //nolint:goerr113 //no specific handling expected
	}

	cred, err := azclients.getTokenCredential()
	if err != nil {
		return nil, fmt.Errorf("failed to get TokenCredential: %w", err)
	}
	azclients.blobServiceClient, err = storageservice.NewClient(fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccountName), cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob service client: %w", err)
	}

	return azclients.blobServiceClient, nil
}
