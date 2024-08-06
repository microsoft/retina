// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package clients

import (
	"sigs.k8s.io/cloud-provider-azure/pkg/azclient"
)

// AzureConfig represents the configuration for Azure clients.
type AzureConfig struct {
	azclient.AzureAuthConfig `json:",inline" yaml:",inline"`
	azclient.ARMClientConfig `json:",inline" yaml:",inline"`

	SubscriptionID string `json:"subscriptionID,omitempty"`
	// The name of the resource group that the cluster is deployed in
	ResourceGroup string `json:"resourceGroup,omitempty" yaml:"resourceGroup,omitempty"`
	// The location of the resource group that the cluster is deployed in
	Location string `json:"location,omitempty" yaml:"location,omitempty"`
}
