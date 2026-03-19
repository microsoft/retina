// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package common contains shared constants, paths, and configuration used across e2ev3 tests.
package common

import (
	"context"
	"flag"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	NetObsRGtag         = "-e2e-netobs-"
	KubeSystemNamespace  = "kube-system"
	TestPodNamespace     = "kube-system-test"
)

var (
	AzureLocations = []string{"eastus2", "northeurope", "uksouth", "centralindia", "westus2"}
	Architectures  = []string{"amd64", "arm64"}
	CreateInfra    = flag.Bool("create-infra", true, "create a Resource group, vNET and AKS cluster for testing")
	DeleteInfra    = flag.Bool("delete-infra", true, "delete a Resource group, vNET and AKS cluster for testing")
	KubeConfig     = flag.String("kubeConfig", "", "Path to kubeconfig file")
)

var (
	RetinaChartPath = func(rootDir string) string {
		return filepath.Join(rootDir, "deploy", "standard", "manifests", "controller", "helm", "retina")
	}
	HubbleChartPath = func(rootDir string) string {
		return filepath.Join(rootDir, "deploy", "hubble", "manifests", "controller", "helm", "retina")
	}
	RetinaAdvancedProfilePath = func(rootDir string) string {
		return filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	}
	KubeConfigFilePath = func(rootDir string) string {
		return filepath.Join(rootDir, "test", "e2e", "test.pem")
	}
)

func ClusterNameForE2ETest(t *testing.T, clusterName string) string {
	if clusterName == "" {
		curuser, err := user.Current()
		require.NoError(t, err)
		username := curuser.Username

		if len(username) > 8 {
			username = username[:8]
			t.Logf("Username is too long, truncating to 8 characters: %s", username)
		}
		clusterName = username + NetObsRGtag + strconv.FormatInt(time.Now().Unix(), 10)
		t.Logf("CLUSTER_NAME is not set, generating a random cluster name: %s", clusterName)
	}
	return clusterName
}

const safetyTimeout = 24 * time.Hour

// Context returns a context with a deadline set to the test deadline - 1 min to ensure cleanup.
// If the test deadline is not set, a deadline is set to Now + 24h to prevent the test from running indefinitely.
func Context(t *testing.T) (context.Context, context.CancelFunc) {
	deadline, ok := t.Deadline()
	if !ok {
		t.Log("Test deadline disabled, deadline set to Now + 24h to prevent test from running indefinitely")
		deadline = time.Now().Add(safetyTimeout)
	}

	// Subtract a minute from the deadline to ensure we have time to cleanup
	deadline = deadline.Add(-time.Minute)

	ctx, cancel := context.WithDeadline(context.Background(), deadline) //nolint:all // cancel is reassigned in next line
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	return ctx, cancel
}
