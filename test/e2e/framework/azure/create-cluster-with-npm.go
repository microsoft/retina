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

var (
	ErrResourceNameTooLong = fmt.Errorf("resource name too long")
	ErrEmptyFile           = fmt.Errorf("empty file")
)

const (
	clusterTimeout       = 15 * time.Minute
	clusterCreateTicker  = 30 * time.Second
	pollFrequency        = 5 * time.Second
	AgentARMSKU          = "Standard_D4pls_v5"
	AuxilaryNodeCount    = 1
	AuxilaryARMNodeCount = 2
)

type CreateNPMCluster struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	ClusterName       string
	VnetName          string
	SubnetName        string
	PodCidr           string
	DNSServiceIP      string
	ServiceCidr       string
	PublicIPs         []string
}

func (c *CreateNPMCluster) Prevalidate() error {
	return nil
}

func (c *CreateNPMCluster) Stop() error {
	return nil
}

func (c *CreateNPMCluster) Run() error {
	// Start with default cluster template
	npmCluster := GetStarterClusterTemplate(c.Location)

	npmCluster.Properties.NetworkProfile.NetworkPolicy = to.Ptr(armcontainerservice.NetworkPolicyAzure)

	//nolint:appendCombine // separate for verbosity
	npmCluster.Properties.AgentPoolProfiles = append(npmCluster.Properties.AgentPoolProfiles, &armcontainerservice.ManagedClusterAgentPoolProfile{ //nolint:all
		Type: to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
		// AvailabilityZones:  []*string{to.Ptr("1")},
		Count:              to.Ptr[int32](AuxilaryNodeCount),
		EnableNodePublicIP: to.Ptr(false),
		Mode:               to.Ptr(armcontainerservice.AgentPoolModeUser),
		OSType:             to.Ptr(armcontainerservice.OSTypeWindows),
		OSSKU:              to.Ptr(armcontainerservice.OSSKUWindows2022),
		ScaleDownMode:      to.Ptr(armcontainerservice.ScaleDownModeDelete),
		VMSize:             to.Ptr(AgentSKU),
		Name:               to.Ptr("ws22"),
		MaxPods:            to.Ptr(int32(MaxPodsPerNode)),
	})

	//nolint:appendCombine // separate for verbosity
	npmCluster.Properties.AgentPoolProfiles = append(npmCluster.Properties.AgentPoolProfiles, &armcontainerservice.ManagedClusterAgentPoolProfile{
		Type:               to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
		AvailabilityZones:  []*string{to.Ptr("1")},
		Count:              to.Ptr[int32](AuxilaryNodeCount),
		EnableNodePublicIP: to.Ptr(false),
		EnableFIPS:         to.Ptr(true),
		Mode:               to.Ptr(armcontainerservice.AgentPoolModeUser),
		OSType:             to.Ptr(armcontainerservice.OSTypeLinux),
		OSSKU:              to.Ptr(armcontainerservice.OSSKUAzureLinux),
		ScaleDownMode:      to.Ptr(armcontainerservice.ScaleDownModeDelete),
		VMSize:             to.Ptr(AgentSKU),
		Name:               to.Ptr("azlinux"),
		MaxPods:            to.Ptr(int32(MaxPodsPerNode)),
	})

	//nolint:appendCombine // separate for verbosity
	npmCluster.Properties.AgentPoolProfiles = append(npmCluster.Properties.AgentPoolProfiles, &armcontainerservice.ManagedClusterAgentPoolProfile{ //nolint:all
		Type: to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
		// AvailabilityZones:  []*string{to.Ptr("1")},
		Count:              to.Ptr[int32](AuxilaryARMNodeCount),
		EnableNodePublicIP: to.Ptr(false),
		Mode:               to.Ptr(armcontainerservice.AgentPoolModeUser),
		OSType:             to.Ptr(armcontainerservice.OSTypeLinux),
		ScaleDownMode:      to.Ptr(armcontainerservice.ScaleDownModeDelete),
		VMSize:             to.Ptr(AgentARMSKU),
		Name:               to.Ptr("arm64"),
		MaxPods:            to.Ptr(int32(MaxPodsPerNode)),
	})

	npmCluster.Properties.AutoUpgradeProfile = &armcontainerservice.ManagedClusterAutoUpgradeProfile{
		NodeOSUpgradeChannel: to.Ptr(armcontainerservice.NodeOSUpgradeChannelNodeImage),
	}

	if len(c.PublicIPs) > 0 {
		publicIPIDs := make([]*armcontainerservice.ResourceReference, 0, len(c.PublicIPs))

		for _, ipID := range c.PublicIPs {
			fmt.Printf("Adding Public IP ID: %s\n", ipID)
			publicIPIDs = append(publicIPIDs, &armcontainerservice.ResourceReference{
				ID: to.Ptr(ipID),
			})
		}

		for _, ip := range c.PublicIPs {
			fmt.Printf("Public IP ID: %s\n", ip)
		}

		if npmCluster.Properties.NetworkProfile.LoadBalancerProfile == nil {
			npmCluster.Properties.NetworkProfile.LoadBalancerProfile = &armcontainerservice.ManagedClusterLoadBalancerProfile{
				OutboundIPs: &armcontainerservice.ManagedClusterLoadBalancerProfileOutboundIPs{
					PublicIPs: publicIPIDs,
				},
			}
		}
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

	poller, err := clientFactory.NewManagedClustersClient().BeginCreateOrUpdate(ctx, c.ResourceGroupName, c.ClusterName, npmCluster, nil)
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
