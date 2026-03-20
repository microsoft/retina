// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package config

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

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

// Flags parsed from test command line.
var (
	CreateInfra = flag.Bool("create-infra", true, "create infrastructure for testing")
	DeleteInfra = flag.Bool("delete-infra", true, "delete infrastructure after testing")
	KubeConfig  = flag.String("kubeconfig", "", "path to kubeconfig file")
	Provider    = flag.String("provider", "azure", "infrastructure provider: azure or kind")
)

const (
	KubeSystemNamespace = "kube-system"
	TestPodNamespace    = "kube-system-test"
	safetyTimeout       = 24 * time.Hour
)

// Architectures lists the CPU architectures to test across.
// Kind clusters are single-arch (amd64), so arm64 is only tested on Azure.
var Architectures []string

// E2EParams holds shared mutable state populated by early pipeline steps
// and read by later ones. Safe under flow.Pipe because steps run sequentially.
type E2EParams struct {
	Cfg   *E2EConfig
	Paths *Paths
}

// Paths returns resolved filesystem paths relative to the repository root.
type Paths struct {
	RootDir    string
	KubeConfig string
	RetinaChart string
	HubbleChart string
	AdvancedProfile string
}

// ResolvePaths computes all standard paths from the repository root directory.
func ResolvePaths(rootDir string) *Paths {
	return &Paths{
		RootDir:         rootDir,
		KubeConfig:      filepath.Join(rootDir, "test", "e2e", "test.pem"),
		RetinaChart:     filepath.Join(rootDir, "deploy", "standard", "manifests", "controller", "helm", "retina"),
		HubbleChart:     filepath.Join(rootDir, "deploy", "hubble", "manifests", "controller", "helm", "retina"),
		AdvancedProfile: filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml"),
	}
}

// TestContext returns a context with a deadline set to the test deadline minus 1 min to ensure cleanup.
// If the test deadline is not set, a deadline is set to Now + 24h to prevent the test from running indefinitely.
func TestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		t.Log("Test deadline disabled, deadline set to Now + 24h to prevent test from running indefinitely")
		deadline = time.Now().Add(safetyTimeout)
	}
	deadline = deadline.Add(-time.Minute)

	ctx, cancel := context.WithDeadline(context.Background(), deadline) //nolint:all // cancel is reassigned in next line
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	return ctx, cancel
}

// DevTag returns a tag derived from git describe, suitable for local dev builds.
func DevTag(rootDir string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	cmd.Dir = rootDir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git describe: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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

	if cfg.Image.Registry == "" {
		cfg.Image.Registry = "ghcr.io"
	}
	if cfg.Image.Namespace == "" {
		cfg.Image.Namespace = "microsoft/retina"
	}

	log.Printf("using image %s/%s:%s", cfg.Image.Registry, cfg.Image.Namespace, cfg.Image.Tag)

	// Kind clusters are single-arch (amd64); Azure supports both.
	if *Provider == "kind" {
		Architectures = []string{"amd64"}
	} else {
		Architectures = []string{"amd64", "arm64"}
	}

	return cfg, nil
}
