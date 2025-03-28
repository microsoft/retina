package params

import (
	"os"
)

var (
	Location           = os.Getenv("LOCATION")
	SubscriptionID     = os.Getenv("AZURE_SUBSCRIPTION_ID")
	ResourceGroup      = os.Getenv("AZURE_RESOURCE_GROUP")
	ClusterName        = os.Getenv("CLUSTER_NAME")
	Nodes              = os.Getenv("NODES")
	NumDeployments     = os.Getenv("NUM_DEPLOYMENTS")
	NumReplicas        = os.Getenv("NUM_REPLICAS")
	NumNetworkPolicies = os.Getenv("NUM_NET_POL")
	CleanUp            = os.Getenv("CLEANUP")
)
