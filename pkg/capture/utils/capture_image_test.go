// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"os"
	"testing"

	"github.com/microsoft/retina/pkg/log"
)

func TestCaptureWorkloadImage(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	logger := log.Logger().Named("TestCaptureWorkloadImage")
	cases := []struct {
		name          string
		env           map[string]string
		sourceVersion string
		versionSource VersionSource
		expectedImage string
	}{
		{
			name:          "Official image",
			env:           map[string]string{},
			sourceVersion: "v1.0.0",
			versionSource: VersionSourceCLIVersion,
			expectedImage: "ghcr.io/microsoft/retina/retina-agent:v1.0.0",
		},
		{
			name: "Debug mode: image determined by CLI version",
			env: map[string]string{
				"DEBUG": "true",
			},
			sourceVersion: "v1.0.0",
			versionSource: VersionSourceCLIVersion,

			expectedImage: "ghcr.io/microsoft/retina/retina-agent:v1.0.0",
		},
		{
			name:          "Debug mode: image determined by environment variable RETINA_AGENT_IMAGE",
			sourceVersion: "v1.0.0",
			env: map[string]string{
				"DEBUG":              "true",
				"RETINA_AGENT_IMAGE": "test.com/retina-agent:v1.0.1",
			},
			versionSource: VersionSourceCLIVersion,
			expectedImage: "test.com/retina-agent:v1.0.1",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			defer func() {
				for k := range tt.env {
					os.Unsetenv(k)
				}
			}()
			debug := false
			if _, ok := tt.env["DEBUG"]; ok {
				debug = true
			}

			actualImage := CaptureWorkloadImage(logger, tt.sourceVersion, debug, VersionSourceCLIVersion)
			if actualImage != tt.expectedImage {
				t.Errorf("Expected image %s, but got %s", tt.expectedImage, actualImage)
			}
		})
	}
}
