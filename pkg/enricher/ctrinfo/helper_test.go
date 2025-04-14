// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package ctrinfo

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/stretchr/testify/require"
)

func TestGetPodInfo(t *testing.T) {
	invalidJSONPath := "invalid_pod_spec.json"
	invalidJSONContent := `{"status": {"metadata": {"name": "retina-pod", "namespace": "retina-namespace"}`

	err := os.WriteFile(invalidJSONPath, []byte(invalidJSONContent), 0o600)
	require.NoError(t, err, "failed to create invalid JSON file")
	defer os.Remove(invalidJSONPath)

	tests := []struct {
		name             string
		ip               string
		podCmdOutput     string
		inspectCmdOutput string
		cmdErr           error
		expectedPodInfo  *cache.PodInfo
		expectedErr      bool
	}{
		{
			name:             "IP found in list of running pods",
			ip:               "10.0.0.4",
			podCmdOutput:     "pod1\npod2\n",
			inspectCmdOutput: "mock_podSpec.json",
			cmdErr:           nil,
			expectedPodInfo:  &cache.PodInfo{Name: "retina-pod", Namespace: "retina-namespace"},
			expectedErr:      false,
		},
		{
			name:             "No IP found in list of running pods",
			ip:               "10.0.0.0",
			podCmdOutput:     "pod1\npod2\n",
			inspectCmdOutput: "mock_podSpec.json",
			cmdErr:           nil,
			expectedPodInfo:  nil,
			expectedErr:      false,
		},
		{
			name:             "Invalid pod spec JSON",
			ip:               "10.0.0.0",
			podCmdOutput:     "pod1\npod2\n",
			inspectCmdOutput: invalidJSONPath,
			cmdErr:           nil,
			expectedPodInfo:  nil,
			expectedErr:      true,
		},
		{
			name:            "Inspect pod error",
			ip:              "10.0.0.0",
			cmdErr:          fmt.Errorf("test error"),
			expectedPodInfo: nil,
			expectedErr:     true,
		},
		{
			name:            "Running pods error",
			ip:              "10.0.0.0",
			podCmdOutput:    "pod1\npod2\n",
			cmdErr:          fmt.Errorf("test error"),
			expectedPodInfo: nil,
			expectedErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crictlCommand = func(command string, args ...string) (string, error) {
				if strings.Contains(args[2], "pods") {
					if tt.podCmdOutput != "" {
						return tt.podCmdOutput, nil
					}
					return "", tt.cmdErr
				}
				if strings.Contains(args[2], "inspectp") {
					if tt.cmdErr != nil {
						return "", tt.cmdErr
					}
					content, err := os.ReadFile(tt.inspectCmdOutput)
					if err != nil {
						return "", err
					}
					return string(content), nil
				}
				return "", fmt.Errorf("unexpected command: %s %v", command, args)
			}

			podInfo, err := GetPodInfo(tt.ip)
			if tt.expectedErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.cmdErr.Error())
				require.Nil(t, podInfo)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPodInfo, podInfo)
			}

			crictlCommand = runCommand
		})
	}
}
