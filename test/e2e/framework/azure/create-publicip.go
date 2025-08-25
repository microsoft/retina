package azure

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5"
)

type CreatePublicIp struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	PublicIpName      string
	IPTagType         string
	Tag               string
}

func (c *CreatePublicIp) Run() error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx := context.Background()
	clientFactory, err := armnetwork.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	log.Printf("creating public ip \"%s\" in resource group \"%s\"...", c.PublicIpName, c.ResourceGroupName)

	poller, err := clientFactory.NewPublicIPAddressesClient().BeginCreateOrUpdate(ctx, c.ResourceGroupName, c.PublicIpName, armnetwork.PublicIPAddress{
		Location: to.Ptr(c.Location),
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			IPTags: []*armnetwork.IPTag{
				{
					IPTagType: to.Ptr(c.IPTagType),
					Tag:       to.Ptr(c.Tag),
				},
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the request for create public ip: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to pull the result for create public ip: %w", err)
	}
	return nil
}

func (c *CreatePublicIp) Prevalidate() error {
	return nil
}

func (c *CreatePublicIp) Stop() error {
	return nil
}
