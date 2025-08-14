// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package statefile

import (
	"os"
	"testing"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/stretchr/testify/require"
)

func TestGetPodInfo(t *testing.T) {
	emptyJSONPath := "empty_azure_vnet.json"
	emptyJSONContent := ``
	err := os.WriteFile(emptyJSONPath, []byte(emptyJSONContent), 0o600)
	require.NoError(t, err, "failed to create empty JSON file")

	invalidJSONPath := "mock_invalid_azure_vnet.json"
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
	err = os.WriteFile(invalidJSONPath, []byte(invalidJSONContent), 0o600)
	require.NoError(t, err, "failed to create invalid JSON file")

	defer os.Remove(emptyJSONPath)
	defer os.Remove(invalidJSONPath)

	tests := []struct {
		name            string
		ip              string
		filePath        string
		expectedPodInfo *cache.PodInfo
		expectedErr     bool
	}{
		{
			name:            "Valid IP match",
			ip:              "192.0.0.5",
			filePath:        "mock_statefile.json",
			expectedPodInfo: &cache.PodInfo{Name: "retina-pod", Namespace: "retina-namespace"},
			expectedErr:     false,
		},
		{
			name:            "No IP match",
			ip:              "10.0.0.0",
			filePath:        "mock_statefile.json",
			expectedPodInfo: nil,
			expectedErr:     false,
		},
		{
			name:            "CNI state file not found",
			ip:              "10.0.0.0",
			filePath:        "non_existent_file.json",
			expectedPodInfo: nil,
			expectedErr:     true,
		},
		{
			name:            "Empty CNI state file",
			ip:              "10.0.0.0",
			filePath:        emptyJSONPath,
			expectedPodInfo: nil,
			expectedErr:     false,
		},
		{
			name:            "Invalid state file JSON",
			ip:              "10.0.0.0",
			filePath:        invalidJSONPath,
			expectedPodInfo: nil,
			expectedErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podInfo, err := GetPodInfo(tt.ip, tt.filePath)

			if tt.expectedErr {
				require.Error(t, err)
				require.Nil(t, podInfo)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPodInfo, podInfo)
			}
		})
	}
}
