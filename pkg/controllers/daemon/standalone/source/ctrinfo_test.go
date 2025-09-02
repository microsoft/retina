// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package source

import (
	"net"
	"os"
	"strings"
	"testing"

	"github.com/microsoft/retina/pkg/common"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kind/pkg/errors"
)

func TestCtrinfoGetAllEndpoints(t *testing.T) {
	invalidJSONPath := "invalid_pod_spec.json"
	invalidJSONContent := `{"status": {"metadata": {"name": "retina-pod", "namespace": "retina-namespace"}`

	err := os.WriteFile(invalidJSONPath, []byte(invalidJSONContent), 0o600)
	require.NoError(t, err, "failed to create invalid JSON file")
	defer os.Remove(invalidJSONPath)

	src := &Ctrinfo{}

	tests := []struct {
		name                   string
		podCmdOutput           string
		inspectCmdOutput       string
		getPodsErr             error
		inspectPodErr          error
		expectedErr            error
		expectedCount          int
		expectedRetinaEndpoint *common.RetinaEndpoint
	}{
		{
			name:             "Successful get all endpoints",
			podCmdOutput:     "pod1\npod2\n",
			inspectCmdOutput: "mock_podSpec.json",
			expectedErr:      nil,
			expectedCount:    2,
			expectedRetinaEndpoint: common.NewRetinaEndpoint(
				"retina-pod",
				"retina-namespace",
				common.NewIPAddress(net.ParseIP("10.0.0.4"), nil),
			),
		},
		{
			name:          "Get all running pods error",
			getPodsErr:    errGetPods,
			expectedErr:   errGetPods,
			expectedCount: 0,
		},
		{
			name:          "Inspect pod command error",
			podCmdOutput:  "pod1\npod2\n",
			inspectPodErr: errInspectPod,
			expectedErr:   errInspectPod,
			expectedCount: 0,
		},
		{
			name:             "Invalid pod spec JSON",
			podCmdOutput:     "pod1\npod2\n",
			inspectCmdOutput: invalidJSONPath,
			expectedErr:      errJSONRead,
			expectedCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalCommand := crictlCommand
			defer func() { crictlCommand = originalCommand }()

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

			endpoints, err := src.GetAllEndpoints()
			if tt.expectedErr != nil {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expectedErr.Error())
				require.Nil(t, endpoints)
			} else {
				require.NoError(t, err)
				require.Len(t, endpoints, tt.expectedCount)
				if tt.expectedCount > 0 {
					require.Equal(t, tt.expectedRetinaEndpoint, endpoints[0])
				}
			}
		})
	}
}
