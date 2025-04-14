// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package statefile

import (
	"fmt"
	"os"
	"testing"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ip       = "192.0.0.5"
	testfile = "mock_statefile.json"
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
		expectedErr     error
	}{
		{
			name:            "Valid IP match",
			ip:              "192.0.0.5",
			filePath:        "mock_statefile.json",
			expectedPodInfo: &cache.PodInfo{Name: "retina-pod", Namespace: "retina-namespace"},
			expectedErr:     nil,
		},
		{
			name:            "No IP match",
			ip:              "10.0.0.0",
			filePath:        "mock_statefile.json",
			expectedPodInfo: nil,
			expectedErr:     nil,
		},
		{
			name:            "CNI state file not found",
			ip:              "10.0.0.0",
			filePath:        "non_existent_file.json",
			expectedPodInfo: nil,
			expectedErr:     fmt.Errorf("open non_existent_file.json: no such file or directory"),
		},
		{
			name:            "Empty CNI state file",
			ip:              "10.0.0.0",
			filePath:        emptyJSONPath,
			expectedPodInfo: nil,
			expectedErr:     nil,
		},
		{
			name:            "Invalid state file JSON",
			ip:              "10.0.0.0",
			filePath:        invalidJSONPath,
			expectedPodInfo: nil,
			expectedErr:     fmt.Errorf("unexpected end of JSON input"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podInfo, err := GetPodInfo(tt.ip, tt.filePath)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				assert.Nil(t, podInfo)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPodInfo, podInfo)
			}
		})
	}
}
