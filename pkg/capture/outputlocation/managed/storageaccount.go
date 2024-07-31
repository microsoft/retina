// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package managed

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"go.uber.org/zap"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/provider/azure/azclients"
)

const (
	defaultConainerName     = "retina-capture"
	storageAccountNamePreix = "retinacapture"

	tagKeyCreatedBy = "createdBy"

	// expiryDurationThreshold defines the minimal duration of the user delegation SAS expiry
	// to let the SAS expiry be tolerable of small capture duration.
	expiryDurationThreshold = 10 * time.Minute
)

func getStorageAccountTag() map[string]*string {
	return map[string]*string{
		tagKeyCreatedBy: to.Ptr("retina"),
	}
}

// getStorageAccountName returns a unique storage account name.
// Limitations of storage account name:
// Storage account name must be between 3 and 24 characters in length and use numbers and lower-case letters only.
func getStorageAccountName() string {
	return storageAccountNamePreix + fmt.Sprintf("%d", time.Now().Unix())
}

// StorageAccountManager manages the lifecycle of the storage account.
type StorageAccountManager struct {
	storageAccountName string

	// uniqueContainerPerNamespace allows unique container namespace for different namespaces.
	uniqueContainerPerNamespace bool

	// containerCreated caches the created container to avoid duplicate creation.
	// Instead of querying the storage account for existing containers, we cache
	// the created containers to reduce the API call to Azure ARM.
	// Because the container creation is idempotent, we don't need a lock around
	// the map when multiple Captures  are created in parallel and enter container
	// creation function.
	containerCreated map[string]struct{}

	azclients azclients.AZClients

	logger *log.ZapLogger
}

func NewStorageAccountManager() *StorageAccountManager {
	return &StorageAccountManager{
		uniqueContainerPerNamespace: true,
		containerCreated:            make(map[string]struct{}),
		logger:                      log.Logger().Named("storageAccount"),
	}
}

func (sam *StorageAccountManager) Init(configFile string) error {
	azclients, err := azclients.NewAZClients(configFile)
	if err != nil {
		return err
	}
	sam.azclients = azclients

	if err := sam.ValidateAuthConfig(); err != nil {
		sam.logger.Error("No all configurations are set, please refer to TODO(add a link)", zap.Error(err))
		return err
	}

	// All setup steps should be idempotent to withstand storage account manager restart.
	ctx := context.Background()
	if err := sam.SetupStorageAccount(ctx); err != nil {
		return err
	}

	if !sam.uniqueContainerPerNamespace {
		if err := sam.CreateBlobContainer(ctx, defaultConainerName); err != nil {
			return err
		}
	}

	return nil
}

func (sam *StorageAccountManager) ValidateAuthConfig() error {

	if len(strings.TrimSpace(sam.azclients.GetClientConfig().ResourceGroup)) == 0 {
		return fmt.Errorf("resourcegroup cannot be empty")
	}
	if len(strings.TrimSpace(sam.azclients.GetClientConfig().Location)) == 0 {
		return fmt.Errorf("location cannot be empty")
	}
	return nil
}

func (sam *StorageAccountManager) SetupStorageAccount(ctx context.Context) error {
	sam.logger.Info("Begin to setup managed storage account")

	resourceGroupName := sam.azclients.GetClientConfig().ResourceGroup
	location := sam.azclients.GetClientConfig().Location

	// Once the storage account is created by the storage account manager, it will be give an unique name
	// and bound to this manager through tags. We'll continue to use the existing one if found.
	existingStorageAccountName, found, err := sam.checkStorageAccountCreated(ctx)
	if err != nil {
		return err
	}
	if found {
		sam.storageAccountName = existingStorageAccountName
		sam.logger.Info("Pick the existing storage account name", zap.String("storage account name", sam.storageAccountName))
	} else {
		sam.storageAccountName = getStorageAccountName()
		sam.logger.Info("Pick a new random storage account name", zap.String("storage account name", sam.storageAccountName))
	}

	if _, err := sam.azclients.CreateBlobServiceClient(sam.storageAccountName); err != nil {
		return err
	}

	sam.logger.Info("Creating the storage account", zap.String("storage account name", sam.storageAccountName))
	resource := &armstorage.AccountCreateParameters{
		Kind: to.Ptr(armstorage.KindStorageV2),
		SKU: &armstorage.SKU{
			Name: to.Ptr(armstorage.SKUNameStandardLRS),
		},
		Location: to.Ptr(location),
		Tags:     getStorageAccountTag(),
		Properties: &armstorage.AccountPropertiesCreateParameters{
			AccessTier: to.Ptr(armstorage.AccessTierCool),
			Encryption: &armstorage.Encryption{
				Services: &armstorage.EncryptionServices{
					Blob: &armstorage.EncryptionService{
						KeyType: to.Ptr(armstorage.KeyTypeAccount),
						Enabled: to.Ptr(true),
					},
				},
				KeySource: to.Ptr(armstorage.KeySourceMicrosoftStorage),
			},
		},
	}
	pollerAccountCreateResp, err := sam.azclients.GetStorageAccountsClient().BeginCreate(ctx, resourceGroupName, sam.storageAccountName, *resource, nil)
	if err != nil {
		return err
	}
	accountCreateResp, err := pollerAccountCreateResp.PollUntilDone(ctx, nil)
	if err != nil {
		sam.logger.Error("Failed to create storage account", zap.String("storage account name", sam.storageAccountName), zap.Error(err))
		return err
	}
	sam.logger.Info("Created storage account", zap.String("storage account name", *accountCreateResp.Account.Name), zap.String("storage account ID", *accountCreateResp.Account.ID))

	managementPolicyRuleName := "auto-delete"
	daysToRetetainBlob := 7
	sam.logger.Info("Creating storage account management policy", zap.String("rule name", managementPolicyRuleName))
	// Create a management policy to enable blob auto-delete
	policy := armstorage.ManagementPolicy{
		Properties: &armstorage.ManagementPolicyProperties{
			Policy: &armstorage.ManagementPolicySchema{
				Rules: []*armstorage.ManagementPolicyRule{
					{
						Name: to.Ptr(managementPolicyRuleName),
						Type: to.Ptr(armstorage.RuleTypeLifecycle),
						Definition: &armstorage.ManagementPolicyDefinition{
							Actions: &armstorage.ManagementPolicyAction{
								BaseBlob: &armstorage.ManagementPolicyBaseBlob{
									Delete: &armstorage.DateAfterModification{
										DaysAfterModificationGreaterThan: to.Ptr(float32(daysToRetetainBlob)),
									},
								},
							},
							Filters: &armstorage.ManagementPolicyFilter{
								BlobTypes: []*string{
									to.Ptr("blockBlob"),
								},
							},
						},
					},
				},
			},
		},
	}
	managementPolicyResp, err := sam.azclients.GetManagementPoliciesClient().CreateOrUpdate(ctx, resourceGroupName, sam.storageAccountName, armstorage.ManagementPolicyNameDefault, policy, nil)
	if err != nil {
		sam.logger.Error("Failed to create management policy", zap.Error(err))
		return err
	}
	sam.logger.Info("Created storage account management policy", zap.String("storage account name", *managementPolicyResp.Name), zap.String("management policy name", *managementPolicyResp.ManagementPolicy.Name), zap.String("management policy ID", *managementPolicyResp.ManagementPolicy.ID))

	sam.logger.Info("Succeeded to setup managed storage account")
	return nil
}

func (sam *StorageAccountManager) CreateBlobContainer(ctx context.Context, containerName string) error {
	if sam.isContainerCreated(containerName) {
		sam.logger.Info("Blob container already created", zap.String("container name", containerName))
		return nil
	}
	sam.logger.Info("Begin to create blob container", zap.String("container name", containerName))

	sam.logger.Info("Creating blob container", zap.String("container name", containerName))
	resourceGroupName := sam.azclients.GetClientConfig().ResourceGroup
	blobConainerCreateResp, err := sam.azclients.GetBlobContainersClient().Create(ctx, resourceGroupName, sam.storageAccountName, containerName, armstorage.BlobContainer{}, nil)
	if err != nil {
		sam.logger.Error("Failed to create container", zap.String("container name", containerName), zap.Error(err))
		return err
	}
	sam.logger.Info("Created blob container", zap.String("blob container name", *blobConainerCreateResp.BlobContainer.Name), zap.String("blo container ID", *blobConainerCreateResp.BlobContainer.ID))

	sam.logger.Info("Creating container immutability policy", zap.String("container name", containerName))
	resp, err := sam.azclients.GetBlobContainersClient().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, sam.storageAccountName, containerName, &armstorage.BlobContainersClientCreateOrUpdateImmutabilityPolicyOptions{
		Parameters: &armstorage.ImmutabilityPolicy{
			Properties: &armstorage.ImmutabilityPolicyProperty{
				AllowProtectedAppendWrites:            to.Ptr(true),
				ImmutabilityPeriodSinceCreationInDays: to.Ptr[int32](4),
			},
		},
	})
	if err != nil {
		return err
	}
	sam.logger.Info("Created container immutability policy", zap.String("container name", containerName), zap.String("immutability policy ID", *resp.ImmutabilityPolicy.ID))

	sam.cacheContainerCreated(containerName)

	sam.logger.Info("Succeeded to create blob container", zap.String("container name", containerName))
	return nil
}

func (sam *StorageAccountManager) isContainerCreated(containerName string) bool {
	_, ok := sam.containerCreated[containerName]
	return ok
}

func (sam *StorageAccountManager) cacheContainerCreated(containerName string) {
	sam.containerCreated[containerName] = struct{}{}
}

func (sam *StorageAccountManager) checkStorageAccountCreated(ctx context.Context) (string, bool, error) {
	retinaAccountTags := getStorageAccountTag()
	resourceGroupName := sam.azclients.GetClientConfig().ResourceGroup
	storageAccountItemsPager := sam.azclients.GetStorageAccountsClient().NewListByResourceGroupPager(resourceGroupName, nil)
	for storageAccountItemsPager.More() {
		pageResp, err := storageAccountItemsPager.NextPage(ctx)
		if err != nil {
			return "", false, err
		}

		for _, account := range pageResp.AccountListResult.Value {
			for k, v := range retinaAccountTags {
				if account.Tags == nil {
					break
				}
				if val, ok := account.Tags[k]; ok && val != nil && *val == *v {
					return *account.Name, true, nil
				}
			}
		}
	}
	return "", false, nil
}

func (sam *StorageAccountManager) ConainerNameByNamespace(namespace string) string {
	if !sam.uniqueContainerPerNamespace {
		return defaultConainerName
	}

	return fmt.Sprintf("%s-%s", defaultConainerName, namespace)
}

// CreateContainerSASURL creates a user delegation SAS URL for the container.
// namespace is to determined the container name, and duration decides the expiry time of the SAS URL.
func (sam *StorageAccountManager) CreateContainerSASURL(ctx context.Context, namespace string, duration time.Duration) (string, error) {
	// create container if not exist
	containerName := sam.ConainerNameByNamespace(namespace)
	if err := sam.CreateBlobContainer(ctx, containerName); err != nil {
		return "", err
	}

	svcClient := sam.azclients.GetBlobServiceClient()

	// Set current and past time and create key
	now := time.Now().UTC().Add(-10 * time.Second)
	// The expiry time of the user delegation SAS is set to 2 times duration of the Capture after the current time.
	expiryDuration := 2 * duration
	if expiryDuration < expiryDurationThreshold {
		expiryDuration = expiryDurationThreshold
	}
	expiry := now.Add(expiryDuration)

	info := service.KeyInfo{
		Start:  to.Ptr(now.UTC().Format(sas.TimeFormat)),
		Expiry: to.Ptr(expiry.UTC().Format(sas.TimeFormat)),
	}

	udc, err := svcClient.GetUserDelegationCredential(context.Background(), info, nil)
	if err != nil {
		return "", err
	}

	// Create Blob Signature Values with desired permissions and sign with user delegation credential
	perms := sas.BlobPermissions{Write: true}
	sasQueryParams, err := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     time.Now().UTC().Add(time.Second * -10),
		ExpiryTime:    time.Now().UTC().Add(expiryDuration),
		Permissions:   perms.String(),
		ContainerName: containerName,
	}.SignWithUserDelegation(udc)
	if err != nil {
		return "", err
	}
	containerSASURL := svcClient.URL() + containerName + "?" + sasQueryParams.Encode()
	sam.logger.Info("Succeeded to create container SAS URL")
	return containerSASURL, nil
}
