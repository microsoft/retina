// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package statefile

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ip       = "192.0.0.5"
	testfile = "/home/beegii/src/retina/pkg/enricher/statefile/mock_statefile.json"
)

func TestFileRead(t *testing.T) {
	StateFileLocation = testfile
	if _, err := os.Stat(StateFileLocation); os.IsNotExist(err) {
		t.Fatalf("mock file does not exist: %v", err)
	}

	podInfo, err := GetPodInfo(ip, "invalid_file_path")
	if podInfo != nil {
		t.Fatalf("expected empty pod info for invalid file path, got: %v", podInfo)
	}
	assert.Error(t, err, "expected error when using invalid file path")
}

func TestFileReadEmpty(t *testing.T) {
	emptyJSONPath := "/home/beegii/src/retina/pkg/plugin/hnsstats/empty_azure_vnet.json"
	emptyJSONContent := ``

	err := os.WriteFile(emptyJSONPath, []byte(emptyJSONContent), 0o600)
	require.NoError(t, err, "failed to create empty JSON file")

	podInfo, err := GetPodInfo(ip, emptyJSONPath)
	if podInfo != nil {
		t.Fatalf("expected no pod info for empty JSON file, got: %v", podInfo)
	}
	require.NoError(t, err)

	// Remove empty JSON file
	err = os.Remove(emptyJSONPath)
	require.NoError(t, err, "failed to remove malformed JSON file")
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
	err := os.WriteFile(malformedJSONPath, []byte(invalidJSONContent), 0o600)
	require.NoError(t, err, "failed to create invalid JSON file")

	podInfo, err := GetPodInfo(ip, malformedJSONPath)
	if podInfo != nil {
		t.Fatalf("expected empty pod info for malformed JSON file, got: %v", podInfo)
	}
	require.Error(t, err, "expected error when decoding invalid JSON")

	// Remove malformed JSON file
	err = os.Remove(malformedJSONPath)
	require.NoError(t, err, "failed to remove malformed JSON file")
}

func TestHnsStatsHelper(t *testing.T) {
	StateFileLocation = testfile
	podInfo, err := GetPodInfo(ip, StateFileLocation)
	require.NoError(t, err)
	assert.Equal(t, "retina2-pod", podInfo.Name)
	assert.Equal(t, "retina2-namespace", podInfo.Namespace)
}
