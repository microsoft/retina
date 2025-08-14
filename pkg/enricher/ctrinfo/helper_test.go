// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package ctrinfo

import (
	"os"
	"strings"
	"testing"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kind/pkg/errors"
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
		getPodsErr       error
		inspectPodErr    error
		expectedErr      error
		expectedPodInfo  *cache.PodInfo
	}{
		{
			name:             "IP found in list of running pods",
			ip:               "10.0.0.4",
			podCmdOutput:     "pod1\npod2\n",
			inspectCmdOutput: "mock_podSpec.json",
			expectedErr:      nil,
			expectedPodInfo:  &cache.PodInfo{Name: "retina-pod", Namespace: "retina-namespace"},
		},
		{
			name:             "No IP found in list of running pods",
			ip:               "10.0.0.0",
			podCmdOutput:     "pod1\npod2\n",
			inspectCmdOutput: "mock_podSpec.json",
			expectedErr:      nil,
			expectedPodInfo:  nil,
		},
		{
			name:             "Invalid pod spec JSON",
			ip:               "10.0.0.4",
			podCmdOutput:     "pod1\npod2\n",
			inspectCmdOutput: invalidJSONPath,
			expectedErr:      errJSONRead,
			expectedPodInfo:  nil,
		},
		{
			name:            "Running pods error",
			ip:              "10.0.0.0",
			getPodsErr:      errGetPods,
			expectedErr:     errGetPods,
			expectedPodInfo: nil,
		},
		{
			name:            "Inspect pod error",
			ip:              "10.0.0.4",
			podCmdOutput:    "pod1\npod2\n",
			inspectPodErr:   errInspectPod,
			expectedErr:     errInspectPod,
			expectedPodInfo: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crictlCommand = func(_ string, args ...string) (string, error) {
				if strings.Contains(args[2], "pods") {
					if tt.getPodsErr != nil {
						return "", tt.getPodsErr
					}
					return tt.podCmdOutput, nil
				}
				if strings.Contains(args[2], "inspectp") {
					if tt.inspectPodErr != nil {
						return "", tt.inspectPodErr
					}
					content, err := os.ReadFile(tt.inspectCmdOutput)
					if err != nil {
						return "", errJSONRead
					}
					return string(content), nil
				}
				return "", errors.New("unknown command")
			}

			podInfo, err := GetPodInfo(tt.ip)
			if tt.expectedErr != nil {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expectedErr.Error())
				require.Nil(t, podInfo)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPodInfo, podInfo)
			}

			crictlCommand = runCommand
		})
	}
}
