// // Copyright (c) Microsoft Corporation.
// // Licensed under the MIT license.

package source

import (
	"net"
	"os"
	"testing"

	"github.com/microsoft/retina/pkg/common"
	"github.com/stretchr/testify/require"
)

func TestStatefileGetAllEndpoints(t *testing.T) {
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

	src := &Statefile{}

	tests := []struct {
		name             string
		filePath         string
		emptyFile        bool
		expectedEndpoint []*common.RetinaEndpoint
		expectedErr      bool
	}{
		{
			name:      "Valid state file",
			filePath:  "mock_statefile.json",
			emptyFile: false,
			expectedEndpoint: []*common.RetinaEndpoint{
				common.NewRetinaEndpoint("retina-pod", "retina-namespace", common.NewIPAddress(net.ParseIP("192.0.0.5"), nil)),
				common.NewRetinaEndpoint("retina-pod2", "retina-namespace2", common.NewIPAddress(net.ParseIP("192.0.0.6"), nil)),
			},
			expectedErr: false,
		},
		{
			name:             "Empty state file",
			filePath:         emptyJSONPath,
			emptyFile:        true,
			expectedEndpoint: nil,
			expectedErr:      false,
		},
		{
			name:             "Missing state file",
			filePath:         "non_existent_file.json",
			emptyFile:        false,
			expectedEndpoint: nil,
			expectedErr:      true,
		},
		{
			name:             "Invalid state file JSON",
			expectedEndpoint: nil,
			emptyFile:        false,
			filePath:         invalidJSONPath,
			expectedErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalPath := StateFileLocation
			StateFileLocation = tt.filePath
			defer func() { StateFileLocation = originalPath }()

			endpoints, err := src.GetAllEndpoints()

			if tt.expectedErr {
				require.Error(t, err)
				require.Nil(t, endpoints)
			} else {
				require.NoError(t, err)
				if tt.emptyFile {
					require.Empty(t, endpoints)
				} else {
					require.ElementsMatch(t, tt.expectedEndpoint, endpoints)
				}
			}
		})
	}
}
