// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package common contains common functions and values that are used across multiple e2e tests.
package common

import (
	"flag"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	RetinaPort int = 10093
	// netObsRGtag is used to tag resources created by this test suite
	NetObsRGtag            = "-e2e-netobs-"
	KubeSystemNamespace    = "kube-system"
	TestPodNamespace       = "kube-system-test"
	AzureAppInsightsKeyEnv = "AZURE_APP_INSIGHTS_KEY"
	OutputFilePathEnv      = "OUTPUT_FILEPATH"
)

var (
	AzureLocations = []string{"eastus2", "northeurope", "uksouth", "centralindia", "westus2"}
	Architectures  = []string{"amd64", "arm64"}
	CreateInfra    = flag.Bool("create-infra", true, "create a Resource group, vNET and AKS cluster for testing")
	DeleteInfra    = flag.Bool("delete-infra", true, "delete a Resource group, vNET and AKS cluster for testing")

	// kubeconfig: path to kubeconfig file, in not provided,
	// a new k8s cluster will be created
	KubeConfig = flag.String("kubeConfig", "", "Path to kubeconfig file")
)

var (
	RetinaChartPath = func(rootDir string) string {
		return filepath.Join(rootDir, "deploy", "legacy", "manifests", "controller", "helm", "retina")
	}
	RetinaAdvancedProfilePath = func(rootDir string) string {
		return filepath.Join(rootDir, "test", "profiles", "advanced", "values.yaml")
	}
	KubeConfigFilePath = func(rootDir string) string {
		return filepath.Join(rootDir, "test", "e2e", "test.pem")
	}
)

func ClusterNameForE2ETest(t *testing.T) string {
	clusterName := os.Getenv("CLUSTER_NAME")
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
