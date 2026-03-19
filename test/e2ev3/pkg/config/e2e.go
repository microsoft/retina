// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

// E2EConfig holds all environment-driven configuration for e2e tests.
type E2EConfig struct {
	Azure AzureConfig
	Image ImageConfig
	Scale ScaleConfig
	Helm  HelmConfig
}

// AzureConfig holds Azure infrastructure settings.
type AzureConfig struct {
	SubscriptionID string
	Location       string
	ResourceGroup  string
	ClusterName    string
}

// ImageConfig holds container image coordinates.
type ImageConfig struct {
	Tag       string
	Namespace string
	Registry  string
}

// ScaleConfig holds scale-test parameters.
type ScaleConfig struct {
	Nodes              string
	NumDeployments     string
	NumReplicas        string
	NumNetworkPolicies string
	CleanUp            string
}

// HelmConfig holds Helm-specific settings.
type HelmConfig struct {
	Driver string
}

// LoadE2EConfig reads environment variables via viper and returns a populated E2EConfig.
func LoadE2EConfig() (*E2EConfig, error) {
	v := viper.New()

	// Bind each env var explicitly — env var names don't match struct field paths.
	bindings := map[string]string{
		"azure.subscriptionid": "AZURE_SUBSCRIPTION_ID",
		"azure.location":       "AZURE_LOCATION",
		"azure.resourcegroup":  "AZURE_RESOURCE_GROUP",
		"azure.clustername":    "CLUSTER_NAME",
		"image.tag":            "TAG",
		"image.namespace":      "IMAGE_NAMESPACE",
		"image.registry":       "IMAGE_REGISTRY",
		"scale.nodes":          "NODES",
		"scale.numdeployments": "NUM_DEPLOYMENTS",
		"scale.numreplicas":    "NUM_REPLICAS",
		"scale.numnetworkpolicies": "NUM_NET_POL",
		"scale.cleanup":        "CLEANUP",
		"helm.driver":          "HELM_DRIVER",
	}

	for key, env := range bindings {
		if err := v.BindEnv(key, env); err != nil {
			return nil, fmt.Errorf("binding env %s to %s: %w", env, key, err)
		}
	}

	// Also accept LOCATION as a fallback for AZURE_LOCATION.
	if v.GetString("azure.location") == "" {
		if err := v.BindEnv("azure.location", "LOCATION"); err != nil {
			return nil, fmt.Errorf("binding env LOCATION: %w", err)
		}
	}

	cfg := &E2EConfig{
		Azure: AzureConfig{
			SubscriptionID: v.GetString("azure.subscriptionid"),
			Location:       v.GetString("azure.location"),
			ResourceGroup:  v.GetString("azure.resourcegroup"),
			ClusterName:    v.GetString("azure.clustername"),
		},
		Image: ImageConfig{
			Tag:       v.GetString("image.tag"),
			Namespace: v.GetString("image.namespace"),
			Registry:  v.GetString("image.registry"),
		},
		Scale: ScaleConfig{
			Nodes:              v.GetString("scale.nodes"),
			NumDeployments:     v.GetString("scale.numdeployments"),
			NumReplicas:        v.GetString("scale.numreplicas"),
			NumNetworkPolicies: v.GetString("scale.numnetworkpolicies"),
			CleanUp:            v.GetString("scale.cleanup"),
		},
		Helm: HelmConfig{
			Driver: v.GetString("helm.driver"),
		},
	}

	if cfg.Image.Tag == "" {
		return nil, fmt.Errorf("TAG env var is required")
	}
	if cfg.Image.Namespace == "" {
		return nil, fmt.Errorf("IMAGE_NAMESPACE env var is required")
	}
	if cfg.Image.Registry == "" {
		return nil, fmt.Errorf("IMAGE_REGISTRY env var is required")
	}

	log.Printf("using image %s/%s:%s", cfg.Image.Registry, cfg.Image.Namespace, cfg.Image.Tag)

	return cfg, nil
}
