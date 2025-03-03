// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package hnsstats

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHnsStatsHelper(t *testing.T) {
	filePath := "/home/beegii/src/retina/pkg/plugin/hnsstats/mock_azure_vnet.json"
	ip := "192.0.0.5"

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("mock file does not exist: %v", err)
	}

	// Scenario 1 - Invalid file path
	podInfo, err := GetPodInfo(ip, "invalid_file_path")
	if podInfo != nil {
		t.Fatalf("expected empty pod info for invalid file path, got: %v", podInfo)
	}
	assert.Error(t, err, "expected error when using invalid file path")

	// Scenario 2 - Unable to decode JSON
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
	err = os.WriteFile(malformedJSONPath, []byte(invalidJSONContent), 0644)
	assert.NoError(t, err, "failed to create invalid JSON file")

	podInfo, err = GetPodInfo(ip, malformedJSONPath)
	if podInfo != nil {
		t.Fatalf("expected empty pod info for malformed JSON file, got: %v", podInfo)
	}
	assert.Error(t, err, "expected error when decoding invalid JSON")

	// Remove malformed JSON file
	err = os.Remove(malformedJSONPath)
	assert.NoError(t, err, "failed to remove malformed JSON file")

	// Scenario 3 - Valid case
	podInfo, err = GetPodInfo(ip, filePath)
	assert.NoError(t, err)
	assert.Equal(t, "retina2-pod", podInfo.Name)
	assert.Equal(t, "retina2-namespace", podInfo.Namespace)
}
