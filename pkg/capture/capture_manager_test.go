// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/microsoft/retina/pkg/capture/provider"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/mock/gomock"
)

func TestCaptureNetwork(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	networkCaptureProvider := provider.NewMockNetworkCaptureProviderInterface(ctrl)
	cm := &CaptureManager{
		networkCaptureProvider: networkCaptureProvider,
		tel:                    telemetry.NewNoopTelemetry(),
	}

	timestamp := file.Now()
	captureName := "capture-name"
	nodeHostName := "node-host-name"
	filter := "-i any"
	duration := 10
	maxSize := 100
	os.Setenv(captureConstants.CaptureNameEnvKey, captureName)
	os.Setenv(captureConstants.NodeHostNameEnvKey, nodeHostName)
	os.Setenv(captureConstants.CaptureStartTimestampEnvKey, file.TimeToString(timestamp))
	os.Setenv(captureConstants.TcpdumpFilterEnvKey, filter)
	os.Setenv(captureConstants.CaptureDurationEnvKey, "10s")
	os.Setenv(captureConstants.CaptureMaxSizeEnvKey, strconv.Itoa(maxSize))

	defer func() {
		os.Unsetenv(captureConstants.CaptureNameEnvKey)
		os.Unsetenv(captureConstants.NodeHostNameEnvKey)
		os.Unsetenv(captureConstants.CaptureStartTimestampEnvKey)
		os.Unsetenv(captureConstants.TcpdumpFilterEnvKey)
		os.Unsetenv(captureConstants.CaptureDurationEnvKey)
		os.Unsetenv(captureConstants.CaptureMaxSizeEnvKey)
	}()

	ctx, cancel := TestContext(t)
	defer cancel()

	tmpFilename := file.CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: timestamp}
	networkCaptureProvider.EXPECT().Setup(tmpFilename).Return(fmt.Sprintf("%s-%s-%s", captureName, nodeHostName, timestamp), nil).Times(1)
	networkCaptureProvider.EXPECT().CaptureNetworkPacket(ctx, filter, duration, maxSize).Return(nil).Times(1)

	_, err := cm.CaptureNetwork(ctx)
	if err != nil {
		t.Errorf("CaptureNetwork should have not fail with error %s", err)
	}
}

func TestEnabledOutputLocation(t *testing.T) {
	cases := []struct {
		name                      string
		env                       map[string]string
		wantEnabledOutputLocation []string
	}{
		{
			name: "HostPath output location is enabled",
			env: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyHostPath): "/tmp/capture",
			},
			wantEnabledOutputLocation: []string{"HostPath"},
		},
		{
			name: "PVC output location is enabled",
			env: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim): "mypvc",
			},
			wantEnabledOutputLocation: []string{"PersistentVolumeClaim"},
		},
		{
			name: "PVC and HostPath output location is enabled",
			env: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyHostPath):              "/tmp/capture",
				string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim): "mypvc",
			},
			wantEnabledOutputLocation: []string{"HostPath", "PersistentVolumeClaim"},
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

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			networkCaptureProvider := provider.NewMockNetworkCaptureProviderInterface(ctrl)

			log.SetupZapLogger(log.GetDefaultLogOpts())
			cm := &CaptureManager{
				networkCaptureProvider: networkCaptureProvider,
				l:                      log.Logger().Named("test"),
			}
			enabledOutputLocations := cm.enabledOutputLocations()
			enabledOutputLocationNames := []string{}
			for _, enabledOutputLocation := range enabledOutputLocations {
				enabledOutputLocationNames = append(enabledOutputLocationNames, enabledOutputLocation.Name())
			}

			if diff := cmp.Diff(tt.wantEnabledOutputLocation, enabledOutputLocationNames); diff != "" {
				t.Errorf("CalculateCaptureTargetsOnNode() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	networkCaptureProvider := provider.NewMockNetworkCaptureProviderInterface(ctrl)
	cm := &CaptureManager{
		networkCaptureProvider: networkCaptureProvider,
	}

	networkCaptureProvider.EXPECT().Cleanup().Return(nil).Times(1)

	if err := cm.Cleanup(); err != nil {
		t.Errorf("Cleanup should have not fail with error %s", err)
	}
}
