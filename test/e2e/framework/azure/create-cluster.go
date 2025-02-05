package azure

import (
	"context"
	"fmt"
	"log"
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
	podCidr           string
	vmSize            string
	networkPluginMode string
	Nodes             int32
}

func (c *CreateCluster) SetPodCidr(podCidr string) *CreateCluster {
	c.podCidr = podCidr
	return c
}

func (c *CreateCluster) SetVMSize(vmSize string) *CreateCluster {
	c.vmSize = vmSize
	return c
}

func (c *CreateCluster) SetNetworkPluginMode(networkPluginMode string) *CreateCluster {
	c.networkPluginMode = networkPluginMode
	return c
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
	if c.Nodes == 0 {
		c.Nodes = MaxNumberOfNodes
	}

	template := GetStarterClusterTemplate(c.Location)

	if c.Nodes > 0 {
		template.Properties.AgentPoolProfiles[0].Count = to.Ptr(c.Nodes)
	}

	if c.podCidr != "" {
		template.Properties.NetworkProfile.PodCidr = to.Ptr(c.podCidr)
	}

	if c.vmSize != "" {
		template.Properties.AgentPoolProfiles[0].VMSize = to.Ptr(c.vmSize)
	}

	if c.networkPluginMode != "" {
		template.Properties.NetworkProfile.NetworkPluginMode = to.Ptr(armcontainerservice.NetworkPluginMode(c.networkPluginMode))
	}

	log.Printf("creating cluster %s in location %s...", c.ClusterName, c.Location)
	poller, err := clientFactory.NewManagedClustersClient().BeginCreateOrUpdate(ctx, c.ResourceGroupName, c.ClusterName, template, nil)
	if err != nil {
		return fmt.Errorf("failed to finish the create cluster request: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to pull the create cluster result: %w", err)
	}
	log.Printf("cluster created %s in location %s...", c.ClusterName, c.Location)

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
					Type: to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
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
