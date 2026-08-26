package azure

import (
	"testing"

	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/stretchr/testify/require"
)

func TestWindowsAgentPool(t *testing.T) {
	pool := windowsAgentPool(200)

	require.Equal(t, int32(200), *pool.Count)
	require.Equal(t, "ws22", *pool.Name)
	require.Equal(t, armcontainerservice.OSTypeWindows, *pool.OSType)
	require.Equal(t, armcontainerservice.OSSKUWindows2022, *pool.OSSKU)
	require.Equal(t, armcontainerservice.AgentPoolModeUser, *pool.Mode)
	require.Equal(t, int32(MaxPodsPerNode), *pool.MaxPods)
	require.Equal(t, AgentWindowsSKU, *pool.VMSize)
}

func TestSetAgentPoolNodeLabels(t *testing.T) {
	pools := GetStarterClusterTemplate("westcentralus").Properties.AgentPoolProfiles
	pools = append(pools, windowsAgentPool(200))
	labelValue := "true"
	labels := map[string]*string{"scale-test": &labelValue}

	setAgentPoolNodeLabels(pools, labels)

	for _, pool := range pools {
		require.Equal(t, "true", *pool.NodeLabels["scale-test"])
	}
}
