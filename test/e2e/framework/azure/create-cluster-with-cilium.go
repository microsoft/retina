package azure

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
)

type CreateCiliumCluster struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	ClusterName       string
}

func (c *CreateCiliumCluster) Prevalidate() error {
	return nil
}

func (c *CreateCiliumCluster) Stop() error {
	return nil
}

func (c *CreateCiliumCluster) Run() error {
	// Start with default cluster template
	ciliumCluster := GetStarterClusterTemplate(c.Location)

	ciliumCluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
	ciliumCluster.Properties.NetworkProfile.NetworkPluginMode = to.Ptr(armcontainerservice.NetworkPluginModeOverlay)
	ciliumCluster.Properties.NetworkProfile.NetworkDataplane = to.Ptr(armcontainerservice.NetworkDataplaneCilium)
	ipv4 := armcontainerservice.IPFamilyIPv4
	ipv6 := armcontainerservice.IPFamilyIPv6
	ciliumCluster.Properties.NetworkProfile.IPFamilies = []*armcontainerservice.IPFamily{&ipv4, &ipv6}

	//nolint:appendCombine // separate for verbosity
	ciliumCluster.Properties.AgentPoolProfiles = append(ciliumCluster.Properties.AgentPoolProfiles, &armcontainerservice.ManagedClusterAgentPoolProfile{ //nolint:all
		Type: to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
		// AvailabilityZones:  []*string{to.Ptr("1")},
		Count:              to.Ptr[int32](AuxilaryNodeCount),
		EnableNodePublicIP: to.Ptr(false),
		Mode:               to.Ptr(armcontainerservice.AgentPoolModeUser),
		OSType:             to.Ptr(armcontainerservice.OSTypeLinux),
		ScaleDownMode:      to.Ptr(armcontainerservice.ScaleDownModeDelete),
		VMSize:             to.Ptr(AgentARMSKU),
		Name:               to.Ptr("arm64"),
		MaxPods:            to.Ptr(int32(MaxPodsPerNode)),
	})

	ciliumCluster.Properties.AutoUpgradeProfile = &armcontainerservice.ManagedClusterAutoUpgradeProfile{
		NodeOSUpgradeChannel: to.Ptr(armcontainerservice.NodeOSUpgradeChannelNodeImage),
	}

	// Deploy cluster
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), clusterTimeout)
	defer cancel()

	clientFactory, err := armcontainerservice.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create az client: %w", err)
	}

	log.Printf("when the cluster is ready, use the below command to access and debug")
	log.Printf("az aks get-credentials --resource-group %s --name %s --subscription %s", c.ResourceGroupName, c.ClusterName, c.SubscriptionID)
	log.Printf("creating cluster \"%s\" in resource group \"%s\"...", c.ClusterName, c.ResourceGroupName)

	poller, err := clientFactory.NewManagedClustersClient().BeginCreateOrUpdate(ctx, c.ResourceGroupName, c.ClusterName, ciliumCluster, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the create cluster request: %w", err)
	}

	notifychan := make(chan struct{})
	go func() {
		_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
			Frequency: pollFrequency,
		})
		if err != nil {
			log.Printf("failed to create cluster: %v\n", err)
		} else {
			log.Printf("cluster %s is ready\n", c.ClusterName)
		}
		close(notifychan)
	}()

	ticker := time.NewTicker(clusterCreateTicker)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to create cluster: %w", ctx.Err())
		case <-ticker.C:
			log.Printf("waiting for cluster %s to be ready...\n", c.ClusterName)
		case <-notifychan:
			if err != nil {
				return fmt.Errorf("received notification, failed to create cluster: %w", err)
			}
			return nil
		}
	}
}
