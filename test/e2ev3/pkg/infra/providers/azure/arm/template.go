// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package arm

import (
	"encoding/json"
	"fmt"

	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/azure"
)

// GenerateTemplate builds a subscription-level ARM template that creates
// all e2e infrastructure in a single deployment: resource group, VNet with
// subnet, public IPs, and AKS cluster.
func GenerateTemplate(cfg *azure.InfraConfig) map[string]any {
	nestedResources := []any{buildVNet(cfg)}

	for _, ip := range cfg.PublicIPs {
		nestedResources = append(nestedResources, buildPublicIP(cfg, ip))
	}

	nestedResources = append(nestedResources, buildAKSCluster(cfg))

	return map[string]any{
		"$schema":        "https://schema.management.azure.com/schemas/2018-05-01/subscriptionDeploymentTemplate.json#",
		"contentVersion": "1.0.0.0",
		"resources": []any{
			buildResourceGroup(cfg),
			buildNestedDeployment(cfg, nestedResources),
		},
	}
}

// GenerateTemplateJSON returns the ARM template as pretty-printed JSON bytes.
func GenerateTemplateJSON(cfg *azure.InfraConfig) ([]byte, error) {
	template := GenerateTemplate(cfg)
	return json.MarshalIndent(template, "", "  ")
}

func buildResourceGroup(cfg *azure.InfraConfig) map[string]any {
	return map[string]any{
		"type":       "Microsoft.Resources/resourceGroups",
		"apiVersion": "2022-09-01",
		"name":       cfg.ResourceGroupName,
		"location":   cfg.Location,
	}
}

func buildNestedDeployment(cfg *azure.InfraConfig, resources []any) map[string]any {
	return map[string]any{
		"type":          "Microsoft.Resources/deployments",
		"apiVersion":    "2022-09-01",
		"name":          "e2e-infra-deployment",
		"resourceGroup": cfg.ResourceGroupName,
		"dependsOn": []string{
			fmt.Sprintf("[resourceId('Microsoft.Resources/resourceGroups', '%s')]", cfg.ResourceGroupName),
		},
		"properties": map[string]any{
			"mode": "Incremental",
			"template": map[string]any{
				"$schema":        "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#",
				"contentVersion": "1.0.0.0",
				"resources":      resources,
			},
		},
	}
}

func buildVNet(cfg *azure.InfraConfig) map[string]any {
	return map[string]any{
		"type":       "Microsoft.Network/virtualNetworks",
		"apiVersion": "2023-04-01",
		"name":       cfg.VnetName,
		"location":   cfg.Location,
		"properties": map[string]any{
			"addressSpace": map[string]any{
				"addressPrefixes": []string{cfg.VnetAddressSpace},
			},
			"flowTimeoutInMinutes": 10,
			"subnets": []map[string]any{
				{
					"name": cfg.SubnetName,
					"properties": map[string]any{
						"addressPrefix": cfg.SubnetAddressSpace,
					},
				},
			},
		},
	}
}

func buildPublicIP(cfg *azure.InfraConfig, ip azure.PublicIPConfig) map[string]any {
	return map[string]any{
		"type":       "Microsoft.Network/publicIPAddresses",
		"apiVersion": "2023-04-01",
		"name":       ip.FullName(cfg.ClusterName),
		"location":   cfg.Location,
		"sku": map[string]any{
			"name": "Standard",
			"tier": "Regional",
		},
		"properties": map[string]any{
			"publicIPAllocationMethod": "Static",
			"publicIPAddressVersion":   ip.IPVersion,
			"ipTags": []map[string]any{
				{
					"ipTagType": "FirstPartyUsage",
					"tag":       "/NonProd",
				},
			},
		},
	}
}

func buildAKSCluster(cfg *azure.InfraConfig) map[string]any {
	subnetRef := fmt.Sprintf("[resourceId('Microsoft.Network/virtualNetworks/subnets', '%s', '%s')]",
		cfg.VnetName, cfg.SubnetName)

	// Agent pool profiles
	pools := make([]map[string]any, 0, len(cfg.AgentPools))
	for _, pool := range cfg.AgentPools {
		p := map[string]any{
			"name":               pool.Name,
			"count":              pool.Count,
			"vmSize":             pool.VMSize,
			"osType":             pool.OSType,
			"mode":               pool.Mode,
			"maxPods":            pool.MaxPods,
			"type":               "VirtualMachineScaleSets",
			"enableNodePublicIP": false,
			"scaleDownMode":      "Delete",
			"vnetSubnetID":       subnetRef,
		}
		if pool.OSSku != "" {
			p["osSku"] = pool.OSSku
		}
		if pool.EnableFIPS {
			p["enableFIPS"] = true
		}
		pools = append(pools, p)
	}

	// Outbound public IP references for load balancer
	outboundIPs := make([]map[string]any, 0, len(cfg.PublicIPs))
	for _, ip := range cfg.PublicIPs {
		outboundIPs = append(outboundIPs, map[string]any{
			"id": fmt.Sprintf("[resourceId('Microsoft.Network/publicIPAddresses', '%s')]",
				ip.FullName(cfg.ClusterName)),
		})
	}

	// Dependencies
	deps := []string{
		fmt.Sprintf("[resourceId('Microsoft.Network/virtualNetworks', '%s')]", cfg.VnetName),
	}
	for _, ip := range cfg.PublicIPs {
		deps = append(deps, fmt.Sprintf("[resourceId('Microsoft.Network/publicIPAddresses', '%s')]",
			ip.FullName(cfg.ClusterName)))
	}

	// Network profile
	networkProfile := map[string]any{
		"networkPlugin":  cfg.NetworkPlugin,
		"loadBalancerSku": "standard",
		"outboundType":   "loadBalancer",
	}
	if cfg.NetworkPolicy != "" {
		networkProfile["networkPolicy"] = cfg.NetworkPolicy
	}
	if cfg.PodCidr != "" {
		networkProfile["podCidr"] = cfg.PodCidr
	}
	if cfg.ServiceCidr != "" {
		networkProfile["serviceCidr"] = cfg.ServiceCidr
	}
	if cfg.DNSServiceIP != "" {
		networkProfile["dnsServiceIP"] = cfg.DNSServiceIP
	}
	if cfg.NetworkPluginMode != "" {
		networkProfile["networkPluginMode"] = cfg.NetworkPluginMode
	}
	if len(outboundIPs) > 0 {
		networkProfile["loadBalancerProfile"] = map[string]any{
			"outboundIPs": map[string]any{
				"publicIPs": outboundIPs,
			},
		}
	}

	// Cluster properties
	properties := map[string]any{
		"dnsPrefix":              cfg.ClusterName,
		"enableRBAC":             cfg.EnableRBAC,
		"enablePodSecurityPolicy": false,
		"agentPoolProfiles":      pools,
		"networkProfile":         networkProfile,
	}

	if cfg.AutoUpgradeChannel != "" {
		properties["autoUpgradeProfile"] = map[string]any{
			"nodeOSUpgradeChannel": cfg.AutoUpgradeChannel,
		}
	}

	// Add Windows profile if any pool is Windows
	for _, pool := range cfg.AgentPools {
		if pool.OSType == "Windows" {
			properties["windowsProfile"] = map[string]any{
				"adminUsername": cfg.WindowsAdminUsername,
				"adminPassword": cfg.WindowsAdminPassword,
			}
			break
		}
	}

	return map[string]any{
		"type":       "Microsoft.ContainerService/managedClusters",
		"apiVersion": "2024-01-01",
		"name":       cfg.ClusterName,
		"location":   cfg.Location,
		"tags": map[string]string{
			"archv2": "",
			"tier":   "production",
		},
		"identity": map[string]any{
			"type": "SystemAssigned",
		},
		"sku": map[string]any{
			"name": "Base",
			"tier": "Standard",
		},
		"properties": properties,
		"dependsOn":  deps,
	}
}
