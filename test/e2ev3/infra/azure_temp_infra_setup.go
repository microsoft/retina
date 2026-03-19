package infra

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/framework/azure"
	"github.com/stretchr/testify/require"
)

const IPPrefix = "serviceTaggedIp"

// CreateAzureTempK8sInfra creates (or connects to) Azure infrastructure for e2e testing.
// Returns the kubeconfig file path.
func CreateAzureTempK8sInfra(ctx context.Context, t *testing.T, rootDir string) string {
	kubeConfigFilePath := common.KubeConfigFilePath(rootDir)
	clusterName := common.ClusterNameForE2ETest(t)

	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	require.NotEmpty(t, subID, "AZURE_SUBSCRIPTION_ID environment variable must be set")

	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(common.AzureLocations))))
		if err != nil {
			t.Fatal("Failed to generate a secure random index", err)
		}
		location = common.AzureLocations[nBig.Int64()]
	}

	rg := os.Getenv("AZURE_RESOURCE_GROUP")
	if rg == "" {
		rg = clusterName
	}

	createWF := createTestInfraWorkflow(subID, rg, clusterName, location, kubeConfigFilePath, *common.CreateInfra)
	require.NoError(t, createWF.Do(ctx), "failed to create test infrastructure")

	t.Cleanup(func() {
		deleteWF := deleteTestInfraWorkflow(subID, rg, location, *common.DeleteInfra)
		if err := deleteWF.Do(context.Background()); err != nil {
			t.Logf("Failed to delete test infrastructure: %v", err)
		}
	})

	return kubeConfigFilePath
}

func createTestInfraWorkflow(subID, rg, clusterName, location, kubeConfigFilePath string, createInfra bool) *flow.Workflow {
	wf := new(flow.Workflow)

	publicIPID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/publicIPAddresses", subID, clusterName)
	publicIPv4FullName := fmt.Sprintf("%s/%s-%s-v4", publicIPID, IPPrefix, clusterName)
	publicIPv6FullName := fmt.Sprintf("%s/%s-%s-v6", publicIPID, IPPrefix, clusterName)

	if createInfra {
		wf.Add(flow.Pipe(
			&azure.CreateResourceGroup{
				SubscriptionID:    subID,
				ResourceGroupName: rg,
				Location:          location,
			},
			&azure.CreateVNet{
				SubscriptionID:    subID,
				ResourceGroupName: rg,
				Location:          location,
				VnetName:          "testvnet",
				VnetAddressSpace:  "10.0.0.0/9",
			},
			&azure.CreateSubnet{
				SubscriptionID:     subID,
				ResourceGroupName:  rg,
				Location:           location,
				VnetName:           "testvnet",
				SubnetName:         "testsubnet",
				SubnetAddressSpace: "10.0.0.0/12",
			},
			&azure.CreatePublicIP{
				SubscriptionID:    subID,
				ResourceGroupName: rg,
				Location:          location,
				ClusterName:       clusterName,
				IPVersion:         string(armnetwork.IPVersionIPv4),
				IPPrefix:          IPPrefix,
			},
			&azure.CreatePublicIP{
				SubscriptionID:    subID,
				ResourceGroupName: rg,
				Location:          location,
				ClusterName:       clusterName,
				IPVersion:         string(armnetwork.IPVersionIPv6),
				IPPrefix:          IPPrefix,
			},
			&azure.CreateNPMCluster{
				SubscriptionID:    subID,
				ResourceGroupName: rg,
				Location:          location,
				ClusterName:       clusterName,
				VnetName:          "testvnet",
				SubnetName:        "testsubnet",
				PodCidr:           "10.128.0.0/9",
				DNSServiceIP:      "192.168.0.10",
				ServiceCidr:       "192.168.0.0/28",
				PublicIPs: []string{
					publicIPv4FullName,
					publicIPv6FullName,
				},
			},
			&azure.GetAKSKubeConfig{
				SubscriptionID:     subID,
				ResourceGroupName:  rg,
				Location:           location,
				ClusterName:        clusterName,
				KubeConfigFilePath: kubeConfigFilePath,
			},
		))
	} else {
		wf.Add(flow.Step(&azure.GetAKSKubeConfig{
			KubeConfigFilePath: kubeConfigFilePath,
			ClusterName:        clusterName,
			SubscriptionID:     subID,
			ResourceGroupName:  rg,
			Location:           location,
		}))
	}

	return wf
}

func deleteTestInfraWorkflow(subID, rg, location string, deleteInfra bool) *flow.Workflow {
	wf := new(flow.Workflow)

	if deleteInfra {
		wf.Add(flow.Step(&azure.DeleteResourceGroup{
			SubscriptionID:    subID,
			ResourceGroupName: rg,
			Location:          location,
		}))
	}

	return wf
}
