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

var (
	errInspectPod     = fmt.Errorf("failed to inspect pod information")
	errGetRunningPods = fmt.Errorf("Failed to get running pods")
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
		expectedErrMsg   string
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
			expectedErrMsg:  errInspectPod.Error(),
		},
		{
			name:            "Running pods error",
			ip:              "10.0.0.0",
			podCmdOutput:    "pod1\npod2\n",
			cmdErr:          fmt.Errorf("test error"),
			expectedPodInfo: nil,
			expectedErr:     true,
			expectedErrMsg:  errGetRunningPods.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crictlCommand = func(command string, args ...string) (string, error) {
				if strings.Contains(args[2], "pods") {
					if tt.cmdErr != nil {
						return "", tt.cmdErr
					}
					return tt.podCmdOutput, nil
				}
				if strings.Contains(args[2], "inspectp") {
					if tt.cmdErr != nil {
						return "", tt.cmdErr
					}
					content, err := os.ReadFile(tt.inspectCmdOutput)
					if err != nil {
						return "", fmt.Errorf("failed to read file: %w", err)
					}
					return string(content), nil
				}
				return "", fmt.Errorf("unexpected command: %s %v", command, args)
			}

			podInfo, err := GetPodInfo(tt.ip)
			if tt.expectedErr {
				require.Error(t, err)
				if tt.expectedErrMsg != "" {
					require.Contains(t, err.Error(), tt.expectedErrMsg)
				}
				require.Nil(t, podInfo)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPodInfo, podInfo)
			}

			crictlCommand = runCommand
		})
	}
}
