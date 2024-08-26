package aws

import (
	"fmt"

	"github.com/weaveworks/eksctl/pkg/ctl/cmdutils"
	"github.com/weaveworks/eksctl/pkg/eks"
)

type CreateCluster struct {
	AccountID   string
	Region      string
	ClusterName string
}

func (c *CreateCluster) Run() {

	// Initialize the command line utility
	ctl := cmdutils.NewCtl()

	// Create a new cluster configuration
	clusterConfig := eks.NewClusterConfig()

	// Set cluster name and region
	clusterConfig.Metadata.Name = c.ClusterName
	clusterConfig.Metadata.Region = c.Region

	// Set node group configuration
	nodeGroup := &eks.NodeGroup{
		Name:            "standard-workers",
		InstanceType:    "t2.medium",
		DesiredCapacity: 3,
		MinSize:         1,
		MaxSize:         4,
	}
	clusterConfig.NodeGroups = []*eks.NodeGroup{nodeGroup}

	// Create the cluster
	if err := ctl.CreateCluster(clusterConfig); err != nil {
		fmt.Printf("Failed to create cluster: %v\n", err)
		return
	}

	fmt.Println("Cluster created successfully!")
}
