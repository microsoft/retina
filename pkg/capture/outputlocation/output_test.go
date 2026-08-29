// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

func TestEnabled(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	cases := []struct {
		name           string
		env            map[string]string
		enabled        bool
		outputLocation Location
	}{
		{
			name: "HostPath output location is enabled",
			env: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyHostPath): "/tmp/capture",
			},
			enabled:        true,
			outputLocation: NewHostPath(log.Logger().Named(string(captureConstants.CaptureOutputLocationEnvKeyHostPath))),
		},
		{
			name:           "HostPath output location is disabled",
			env:            map[string]string{},
			enabled:        false,
			outputLocation: NewHostPath(log.Logger().Named(string(captureConstants.CaptureOutputLocationEnvKeyHostPath))),
		},
		{
			name: "PVC output location is enabled",
			env: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim): "mypvc",
			},
			enabled:        true,
			outputLocation: NewPersistentVolumeClaim(log.Logger().Named(string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim))),
		},
		{
			name:           "PVC output location is disabled",
			env:            map[string]string{},
			enabled:        false,
			outputLocation: NewPersistentVolumeClaim(log.Logger().Named(string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim))),
		},
		{
			name:           "Blob output location is disabled",
			env:            map[string]string{},
			enabled:        false,
			outputLocation: NewBlobUpload(log.Logger().Named("blob")),
		},
		{
			name:           "S3 output location is disabled",
			env:            map[string]string{},
			enabled:        false,
			outputLocation: NewS3Upload(log.Logger().Named("s3")),
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

			isEnabled := tt.outputLocation.Enabled()
			assert.Equal(t, isEnabled, tt.enabled, "Failed enablement check for output location")
		})
	}
}

func TestOutput(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	cases := []struct {
		name           string
		env            map[string]string
		outputLocation Location
		srcPath        string
		hasError       bool
	}{
		{
			name: "HostPath output failed on no src file",
			env: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyHostPath): "out",
			},
			outputLocation: NewHostPath(log.Logger().Named(string(captureConstants.CaptureOutputLocationEnvKeyHostPath))),
			srcPath:        "src.out",
			hasError:       true,
		},
		{
			name: "PVC output failed on no src file",
			env: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim): "out",
			},
			outputLocation: NewPersistentVolumeClaim(log.Logger().Named(string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim))),
			srcPath:        "src.out",
			hasError:       true,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

			err := tt.outputLocation.Output(ctx, tt.srcPath)
			assert.Equal(t, tt.hasError, err != nil, "Output check failed on source file open")
		})
	}
}
