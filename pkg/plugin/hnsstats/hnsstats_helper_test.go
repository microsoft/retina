// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package hnsstats

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	mockAzureVnetPath = "/home/beegii/src/retina/pkg/plugin/hnsstats/mock_azure_vnet.json"
	ip                = "192.0.0.5"
)

func TestFileRead(t *testing.T) {
	if _, err := os.Stat(mockAzureVnetPath); os.IsNotExist(err) {
		t.Fatalf("mock file does not exist: %v", err)
	}

	podInfo, err := GetPodInfo(ip, "invalid_file_path")
	if podInfo != nil {
		t.Fatalf("expected empty pod info for invalid file path, got: %v", podInfo)
	}
	assert.Error(t, err, "expected error when using invalid file path")
}

func TestJsonDecode(t *testing.T) {
	malformedJSONPath := "/home/beegii/src/retina/pkg/plugin/hnsstats/mock_invalid_azure_vnet.json"
	invalidJSONContent := `{
		"Network": {
			"ExternalInterfaces": {
				"eth0": {
					"Networks": {
						"192.0.0.5": {
							"IpAddresses": [
								{
									"IP": "192.0.0.5"
								}
							],
							"PodName": "retina2-pod",
							"PodNamespace": "retina2-namespace"
						}
					}
				}
			}
		}
	`

	// Write malformed JSON into file
	err := os.WriteFile(malformedJSONPath, []byte(invalidJSONContent), 0644)
	assert.NoError(t, err, "failed to create invalid JSON file")

	podInfo, err := GetPodInfo(ip, malformedJSONPath)
	if podInfo != nil {
		t.Fatalf("expected empty pod info for malformed JSON file, got: %v", podInfo)
	}
	assert.Error(t, err, "expected error when decoding invalid JSON")

	// Remove malformed JSON file
	err = os.Remove(malformedJSONPath)
	assert.NoError(t, err, "failed to remove malformed JSON file")

	podInfo, err = GetPodInfo(ip, mockAzureVnetPath)
	assert.NoError(t, err)
	assert.Equal(t, "retina2-pod", podInfo.Name)
	assert.Equal(t, "retina2-namespace", podInfo.Namespace)
}

func TestHnsStatsHelper(t *testing.T) {
	podInfo, err := GetPodInfo(ip, mockAzureVnetPath)
	assert.NoError(t, err)
	assert.Equal(t, "retina2-pod", podInfo.Name)
	assert.Equal(t, "retina2-namespace", podInfo.Namespace)
}
