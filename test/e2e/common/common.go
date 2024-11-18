// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package common contains common functions and values that are used across multiple e2e tests.
package common

import (
	"os"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	RetinaPort int = 10093
	// netObsRGtag is used to tag resources created by this test suite
	NetObsRGtag         = "-e2e-netobs-"
	KubeSystemNamespace = "kube-system"
	TestPodNamespace    = "kube-system-test"
)

var AzureLocations = []string{"eastus2", "northeurope", "uksouth", "centralindia", "westus2"}

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
