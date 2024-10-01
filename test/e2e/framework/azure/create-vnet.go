package azure

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

const FlowTimeoutInMinutes = 10

type CreateVNet struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	VnetName          string
	VnetAddressSpace  string
}

func (c *CreateVNet) Run(_ *types.RuntimeObjects) error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx := context.Background()
	clientFactory, err := armnetwork.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	log.Printf("creating vnet \"%s\" in resource group \"%s\"...", c.VnetName, c.ResourceGroupName)

	poller, err := clientFactory.NewVirtualNetworksClient().BeginCreateOrUpdate(ctx, c.ResourceGroupName, c.VnetName, armnetwork.VirtualNetwork{
		Location: to.Ptr(c.Location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{
					to.Ptr(c.VnetAddressSpace),
				},
			},
			FlowTimeoutInMinutes: to.Ptr[int32](FlowTimeoutInMinutes),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the request for create vnet: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to pull the result for create vnet: %w", err)
	}
	return nil
}

func (c *CreateVNet) PreRun() error {
	return nil
}

func (c *CreateVNet) Stop() error {
	return nil
}

type CreateSubnet struct {
	SubscriptionID     string
	ResourceGroupName  string
	Location           string
	VnetName           string
	SubnetName         string
	SubnetAddressSpace string
}

func (c *CreateSubnet) Run(_ *types.RuntimeObjects) error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx := context.Background()
	clientFactory, err := armnetwork.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	log.Printf("creating subnet \"%s\" in vnet \"%s\" in resource group \"%s\"...", c.SubnetName, c.VnetName, c.ResourceGroupName)

	poller, err := clientFactory.NewSubnetsClient().BeginCreateOrUpdate(ctx, c.ResourceGroupName, c.VnetName, c.SubnetName, armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr(c.SubnetAddressSpace),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the request for create subnet: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to pull the result for create subnet: %w", err)
	}
	return nil
}

func (c *CreateSubnet) PreRun() error {
	return nil
}

func (c *CreateSubnet) Stop() error {
	return nil
}
