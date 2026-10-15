package azure

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

const (
	resourceGroupDeletionTimeout        = 30 * time.Minute
	resourceGroupDeletionInitialBackoff = 5 * time.Second
	resourceGroupDeletionMaxBackoff     = time.Minute
)

type DeleteResourceGroup struct {
	SubscriptionID    string
	ResourceGroupName string
	ClusterName       string
	Location          string
}

func (d *DeleteResourceGroup) Run() error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	resourceClientFactory, err := armresources.NewClientFactory(d.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}
	clusterClientFactory, err := armcontainerservice.NewClientFactory(d.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create managed cluster client: %w", err)
	}

	resourceGroupsClient := resourceClientFactory.NewResourceGroupsClient()
	managedClustersClient := clusterClientFactory.NewManagedClustersClient()
	operations := resourceGroupDeletionOperations{
		beginDelete: func(ctx context.Context, resourceGroupName string) (pollResourceGroupDeletion, error) {
			forceDeleteType := "Microsoft.Compute/virtualMachines,Microsoft.Compute/virtualMachineScaleSets"
			poller, err := resourceGroupsClient.BeginDelete(ctx, resourceGroupName, &armresources.ResourceGroupsClientBeginDeleteOptions{
				ForceDeletionTypes: to.Ptr(forceDeleteType),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to begin deleting resource group %q: %w", resourceGroupName, err)
			}
			return func(ctx context.Context) error {
				_, err := poller.PollUntilDone(ctx, nil)
				if err != nil {
					return fmt.Errorf("failed to poll deletion of resource group %q: %w", resourceGroupName, err)
				}
				return nil
			}, nil
		},
		resourceGroupExists: func(ctx context.Context, resourceGroupName string) (bool, error) {
			_, err := resourceGroupsClient.Get(ctx, resourceGroupName, nil)
			if isNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, fmt.Errorf("failed to get resource group %q: %w", resourceGroupName, err)
			}
			return true, nil
		},
		getNodeResourceGroup: func(ctx context.Context) (string, error) {
			if d.ClusterName == "" {
				return "", nil
			}
			cluster, err := managedClustersClient.Get(ctx, d.ResourceGroupName, d.ClusterName, nil)
			if isNotFound(err) {
				return "", nil
			}
			if err != nil {
				return "", fmt.Errorf("failed to get managed cluster %q: %w", d.ClusterName, err)
			}
			if cluster.Properties == nil || cluster.Properties.NodeResourceGroup == nil {
				return "", nil
			}
			return *cluster.Properties.NodeResourceGroup, nil
		},
		wait: waitForResourceGroupDeletionRetry,
	}

	ctx, cancel := context.WithTimeout(context.Background(), resourceGroupDeletionTimeout)
	defer cancel()

	return deleteResourceGroups(ctx, d.ResourceGroupName, operations)
}

func (d *DeleteResourceGroup) Prevalidate() error {
	return nil
}

func (d *DeleteResourceGroup) Stop() error {
	return nil
}

type pollResourceGroupDeletion func(context.Context) error

type resourceGroupDeletionOperations struct {
	beginDelete          func(context.Context, string) (pollResourceGroupDeletion, error)
	resourceGroupExists  func(context.Context, string) (bool, error)
	getNodeResourceGroup func(context.Context) (string, error)
	wait                 func(context.Context, time.Duration) error
}

func deleteResourceGroups(ctx context.Context, resourceGroupName string, operations resourceGroupDeletionOperations) error {
	nodeResourceGroup, err := operations.getNodeResourceGroup(ctx)
	if err != nil {
		return fmt.Errorf("failed to get the AKS node resource group before deleting %q: %w", resourceGroupName, err)
	}
	if nodeResourceGroup != "" {
		log.Printf("AKS cluster uses node resource group %q", nodeResourceGroup)
	}

	if err := deleteResourceGroupWithRetry(ctx, resourceGroupName, operations); err != nil {
		return err
	}
	if nodeResourceGroup == "" {
		return nil
	}

	return deleteResourceGroupWithRetry(ctx, nodeResourceGroup, operations)
}

func deleteResourceGroupWithRetry(ctx context.Context, resourceGroupName string, operations resourceGroupDeletionOperations) error {
	backoff := resourceGroupDeletionInitialBackoff
	var lastErr error

	for attempt := 1; ; attempt++ {
		exists, err := operations.resourceGroupExists(ctx, resourceGroupName)
		switch {
		case err != nil:
			lastErr = fmt.Errorf("failed to check whether resource group %q exists: %w", resourceGroupName, err)
		case !exists:
			log.Printf("resource group %q deleted successfully", resourceGroupName)
			return nil
		default:
			log.Printf("requesting deletion of resource group %q (attempt %d)...", resourceGroupName, attempt)
			poll, beginErr := operations.beginDelete(ctx, resourceGroupName)
			if beginErr != nil {
				lastErr = fmt.Errorf("azure did not accept deletion of resource group %q: %w", resourceGroupName, beginErr)
				break
			}

			log.Printf("Azure accepted deletion of resource group %q", resourceGroupName)
			pollErr := poll(ctx)
			if pollErr == nil {
				log.Printf("resource group %q deleted successfully", resourceGroupName)
				return nil
			}
			lastErr = fmt.Errorf("azure accepted deletion of resource group %q, but deletion did not complete: %w", resourceGroupName, pollErr)
		}

		if ctx.Err() != nil {
			return fmt.Errorf("timed out waiting for resource group %q deletion after %s: %w", resourceGroupName, resourceGroupDeletionTimeout, lastErr)
		}
		if !isTransientResourceGroupDeletionError(lastErr) {
			return lastErr
		}

		log.Printf("transient resource group deletion failure: %v; retrying in %s", lastErr, backoff)
		if err := operations.wait(ctx, backoff); err != nil {
			return fmt.Errorf("timed out waiting to retry resource group %q deletion after %s: %w", resourceGroupName, resourceGroupDeletionTimeout, lastErr)
		}
		backoff *= 2
		if backoff > resourceGroupDeletionMaxBackoff {
			backoff = resourceGroupDeletionMaxBackoff
		}
	}
}

func isTransientResourceGroupDeletionError(err error) bool {
	var responseErr *azcore.ResponseError
	if errors.As(err, &responseErr) {
		switch responseErr.ErrorCode {
		case "ResourceGroupDeletionBlocked", "AnotherOperationInProgress":
			return true
		}
		switch responseErr.StatusCode {
		case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError,
			http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}

	var networkErr net.Error
	return errors.As(err, &networkErr) && networkErr.Timeout()
}

func isNotFound(err error) bool {
	var responseErr *azcore.ResponseError
	return errors.As(err, &responseErr) && responseErr.StatusCode == http.StatusNotFound
}

func waitForResourceGroupDeletionRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("retry wait canceled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
