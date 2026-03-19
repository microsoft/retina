// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package azure

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

// InfraConfig defines the complete infrastructure configuration for deploying
// all e2e test resources in a single ARM template deployment.
type InfraConfig struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	ClusterName       string

	// VNet configuration
	VnetName           string
	VnetAddressSpace   string
	SubnetName         string
	SubnetAddressSpace string

	// Cluster network configuration
	PodCidr      string
	ServiceCidr  string
	DNSServiceIP string

	// Public IP configuration
	PublicIPs []PublicIPConfig

	// Agent pool configuration
	AgentPools []AgentPoolConfig

	// Cluster configuration
	NetworkPlugin      string
	NetworkPolicy      string
	NetworkPluginMode  string
	EnableRBAC         bool
	AutoUpgradeChannel string

	// Windows node configuration
	WindowsAdminUsername string
	WindowsAdminPassword string
}

// PublicIPConfig defines a public IP address to create.
type PublicIPConfig struct {
	NamePrefix string
	IPVersion  string // "IPv4" or "IPv6"
}

// AgentPoolConfig defines an AKS agent pool.
type AgentPoolConfig struct {
	Name       string
	Count      int32
	VMSize     string
	OSType     string // "Linux" or "Windows"
	OSSku      string // "Windows2022", "AzureLinux", etc. Empty for default.
	Mode       string // "System" or "User"
	MaxPods    int32
	EnableFIPS bool
}

// FullName returns the public IP resource name, e.g. "serviceTaggedIp-mycluster-v4".
func (ip PublicIPConfig) FullName(clusterName string) string {
	suffix := "v4"
	if strings.Contains(ip.IPVersion, "6") {
		suffix = "v6"
	}
	return fmt.Sprintf("%s-%s-%s", ip.NamePrefix, clusterName, suffix)
}

// DefaultE2EInfraConfig returns the standard infrastructure configuration
// matching the existing e2e test setup (NPM cluster with 4 agent pools).
func DefaultE2EInfraConfig(subscriptionID, resourceGroupName, location, clusterName string) *InfraConfig {
	return &InfraConfig{
		SubscriptionID:    subscriptionID,
		ResourceGroupName: resourceGroupName,
		Location:          location,
		ClusterName:       clusterName,

		VnetName:           "testvnet",
		VnetAddressSpace:   "10.0.0.0/9",
		SubnetName:         "testsubnet",
		SubnetAddressSpace: "10.0.0.0/12",

		PodCidr:      "10.128.0.0/9",
		ServiceCidr:  "192.168.0.0/28",
		DNSServiceIP: "192.168.0.10",

		PublicIPs: []PublicIPConfig{
			{NamePrefix: "serviceTaggedIp", IPVersion: "IPv4"},
			{NamePrefix: "serviceTaggedIp", IPVersion: "IPv6"},
		},

		AgentPools: []AgentPoolConfig{
			{Name: "nodepool1", Count: 3, VMSize: "Standard_DS4_v2", OSType: "Linux", Mode: "System", MaxPods: 250},
			{Name: "ws22", Count: 1, VMSize: "Standard_DS4_v2", OSType: "Windows", OSSku: "Windows2022", Mode: "User", MaxPods: 250},
			{Name: "azlinux", Count: 1, VMSize: "Standard_D4pls_v5", OSType: "Linux", OSSku: "AzureLinux", Mode: "User", MaxPods: 250, EnableFIPS: true},
			{Name: "arm64", Count: 2, VMSize: "Standard_D4pls_v5", OSType: "Linux", Mode: "User", MaxPods: 250},
		},

		NetworkPlugin:        "azure",
		NetworkPolicy:        "azure",
		EnableRBAC:           true,
		AutoUpgradeChannel:   "node-image",
		WindowsAdminUsername: "azureuser",
		WindowsAdminPassword: generatePassword(),
	}
}

func generatePassword() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Guarantee complexity: uppercase (P), lowercase (w), digit (1), special (!)
	return "Pw" + base64.RawStdEncoding.EncodeToString(b)[:12] + "!1"
}
