//go:build scale || e2e

package retina

import "flag"

var (
	locations   = []string{"eastus2", "centralus", "southcentralus", "uksouth", "centralindia", "westus2"}
	createInfra = flag.Bool("create-infra", true, "create a Resource group, vNET and AKS cluster for testing")
	deleteInfra = flag.Bool("delete-infra", true, "delete a Resource group, vNET and AKS cluster for testing")
)
