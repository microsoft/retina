package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
)

const (
	MaxNumberOfNodes = 3
	MaxPodsPerNode   = 250
	AgentSKU         = "Standard_DS4_v2"
)

var defaultClusterCreateTimeout = 30 * time.Minute

type CreateCluster struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	ClusterName       string
}

func (c *CreateCluster) Run() error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultClusterCreateTimeout)
	defer cancel()
	clientFactory, err := armcontainerservice.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	poller, err := clientFactory.NewManagedClustersClient().BeginCreateOrUpdate(ctx, c.ResourceGroupName, c.ClusterName, GetStarterClusterTemplate(c.Location), nil)
	if err != nil {
		return fmt.Errorf("failed to finish the create cluster request: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to pull the create cluster result: %w", err)
	}

	return nil
}

func GetStarterClusterTemplate(location string) armcontainerservice.ManagedCluster {
	id := armcontainerservice.ResourceIdentityTypeSystemAssigned
	return armcontainerservice.ManagedCluster{
		Location: to.Ptr(location),
		Tags: map[string]*string{
			"archv2": to.Ptr(""),
			"tier":   to.Ptr("production"),
		},
		Properties: &armcontainerservice.ManagedClusterProperties{
			AddonProfiles: map[string]*armcontainerservice.ManagedClusterAddonProfile{},
			/* Moving this to a separate stage to enable AMA since it takes some time to provision
			AzureMonitorProfile: &armcontainerservice.ManagedClusterAzureMonitorProfile{
				Metrics: &armcontainerservice.ManagedClusterAzureMonitorProfileMetrics{
					Enabled: to.Ptr(true),
				},
			},
			*/
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Type:               to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
					// AvailabilityZones:  []*string{to.Ptr("1")},
					Count:              to.Ptr[int32](MaxNumberOfNodes),
					EnableNodePublicIP: to.Ptr(false),
					Mode:               to.Ptr(armcontainerservice.AgentPoolModeSystem),
					OSType:             to.Ptr(armcontainerservice.OSTypeLinux),
					ScaleDownMode:      to.Ptr(armcontainerservice.ScaleDownModeDelete),
					VMSize:             to.Ptr(AgentSKU),
					Name:               to.Ptr("nodepool1"),
					MaxPods:            to.Ptr(int32(MaxPodsPerNode)),
				},
			},
			KubernetesVersion:       to.Ptr(""),
			DNSPrefix:               to.Ptr("dnsprefix1"),
			EnablePodSecurityPolicy: to.Ptr(false),
			EnableRBAC:              to.Ptr(true),
			LinuxProfile:            nil,
			NetworkProfile: &armcontainerservice.NetworkProfile{
				LoadBalancerSKU: to.Ptr(armcontainerservice.LoadBalancerSKUStandard),
				OutboundType:    to.Ptr(armcontainerservice.OutboundTypeLoadBalancer),
				NetworkPlugin:   to.Ptr(armcontainerservice.NetworkPluginAzure),
			},
			WindowsProfile: &armcontainerservice.ManagedClusterWindowsProfile{
				AdminPassword: to.Ptr("replacePassword1234$"),
				AdminUsername: to.Ptr("azureuser"),
			},
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: &id,
		},

		SKU: &armcontainerservice.ManagedClusterSKU{
			Name: to.Ptr(armcontainerservice.ManagedClusterSKUName("Base")),
			Tier: to.Ptr(armcontainerservice.ManagedClusterSKUTierStandard),
		},
	}
}

func (c *CreateCluster) Prevalidate() error {
	return nil
}

func (c *CreateCluster) Stop() error {
	return nil
}
