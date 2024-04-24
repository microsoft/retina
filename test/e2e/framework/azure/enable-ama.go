//go:build e2eframework

package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dashboard/armdashboard"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
)

const fileperms = 0o600

type CreateAzureMonitor struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	ClusterName       string
}

func (c *CreateAzureMonitor) Run() error {
	log.Printf(`this will deploy azure monitor workspace and grafana, but as of 1/9/2024, the api docs don't show how to do 
az aks update --enable-azure-monitor-metrics \
-n $NAME \
-g $CLUSTER_RESOURCE_GROUP \
--azure-monitor-workspace-resource-id $AZMON_RESOURCE_ID \
--grafana-resource-id  $GRAFANA_RESOURCE_ID
`)

	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}

	ctx := context.Background()
	amaClientFactory, err := armmonitor.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create azure monitor workspace client: %w", err)
	}
	log.Printf("creating resource group %s in location %s...", c.ResourceGroupName, c.Location)

	// create azure monitor
	_, err = amaClientFactory.NewAzureMonitorWorkspacesClient().Create(ctx, c.ResourceGroupName, "test", armmonitor.AzureMonitorWorkspaceResource{
		Location: &c.Location,
	}, &armmonitor.AzureMonitorWorkspacesClientCreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to azure monitor workspace: %w", err)
	}

	// Create grafana

	granafaClientFactory, err := armdashboard.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create grafana client: %w", err)
	}

	_, err = granafaClientFactory.NewGrafanaClient().BeginCreate(ctx, c.ResourceGroupName, "test", armdashboard.ManagedGrafana{}, &armdashboard.GrafanaClientBeginCreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create grafana: %w", err)
	}

	log.Printf("azure monitor workspace %s in location %s", c.ResourceGroupName, c.Location)

	// update aks cluster

	ctx, cancel := context.WithTimeout(context.Background(), defaultClusterCreateTimeout)
	defer cancel()
	aksClientFactory, err := armcontainerservice.NewClientFactory(c.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	cluster, err := aksClientFactory.NewManagedClustersClient().Get(ctx, c.ResourceGroupName, c.ClusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster to enable AMA: %w", err)
	}

	// enable Azure Monitor Metrics
	cluster.Properties.AzureMonitorProfile.Metrics.Enabled = to.Ptr(true)

	// Marshal the struct into a JSON byte array with indentation
	jsonData, err := json.MarshalIndent(cluster, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal cluster to JSON for AMA: %w", err)
	}

	// Write the JSON data to a file
	err = os.WriteFile("cluster.json", jsonData, fileperms)
	if err != nil {
		return fmt.Errorf("failed to write cluster JSON to file for AMA: %w", err)
	}

	poller, err := aksClientFactory.NewManagedClustersClient().BeginCreateOrUpdate(ctx, c.ResourceGroupName, c.ClusterName, GetStarterClusterTemplate(c.Location), nil)
	if err != nil {
		return fmt.Errorf("failed to finish the update cluster request for AMA: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to enable AMA on cluster %s: %w", *cluster.Name, err)
	}

	return nil
}

func (c *CreateAzureMonitor) Prevalidate() error {
	return nil
}

func (c *CreateAzureMonitor) Stop() error {
	return nil
}
