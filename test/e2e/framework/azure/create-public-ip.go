package azure

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

type CreatePublicIP struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	ClusterName       string
	IPVersion         string
	IPPrefix          string
}

func (c *CreatePublicIP) Prevalidate() error {
	return nil
}

func (c *CreatePublicIP) Stop() error {
	return nil
}

func (c *CreatePublicIP) Run() error {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), clusterTimeout)
	defer cancel()

	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to create public IP client", err)
	}

	publicIPParams := armnetwork.PublicIPAddress{
		Location: to.Ptr(c.Location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
			Tier: to.Ptr(armnetwork.PublicIPAddressSKUTierRegional),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			PublicIPAddressVersion:   to.Ptr(armnetwork.IPVersion(c.IPVersion)),
			IPTags: []*armnetwork.IPTag{
				{
					IPTagType: to.Ptr("FirstPartyUsage"),
					Tag:       to.Ptr("/NonProd"),
				},
			},
		},
	}

	var version string
	switch c.IPVersion {
	case string(armnetwork.IPVersionIPv4):
		version = "v4"
	case string(armnetwork.IPVersionIPv6):
		version = "v6"
	default:
		return fmt.Errorf("%w: invalid IP version: %s", err, c.IPVersion)
	}

	ipName := fmt.Sprintf("%s-%s-%s", c.IPPrefix, c.ClusterName, version)

	poller, err := publicIPClient.BeginCreateOrUpdate(ctx, c.ResourceGroupName, ipName, publicIPParams, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to create public IP address", err)
	}

	notifychan := make(chan struct{})
	go func() {
		_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
			Frequency: 5 * time.Second,
		})
		if err != nil {
			log.Printf("failed to create Public IP - %s : %v\n", ipName, err)
		} else {
			log.Printf("Public IP %s created\n", ipName)
		}
		close(notifychan)
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to create Public IP: %w", ctx.Err())
		case <-ticker.C:
			log.Printf("waiting for Public IP %s to be ready...\n", ipName)
		case <-notifychan:
			if err != nil {
				return fmt.Errorf("received notification, failed to create public IP address: %w", err)
			}
			return nil
		}
	}
}
