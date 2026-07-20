// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	networkCaptureProvider.EXPECT().CaptureNetworkPacket(ctx, filter, duration, maxSize, 0).Return(nil).Times(1)

	_, err := cm.CaptureNetwork(ctx)
	if err != nil {
		t.Errorf("CaptureNetwork should have not fail with error %s", err)
	}
}

func TestCaptureNetworkWithRotatingCapture(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	networkCaptureProvider := provider.NewMockNetworkCaptureProviderInterface(ctrl)
	cm := &CaptureManager{
		networkCaptureProvider: networkCaptureProvider,
		tel:                    telemetry.NewNoopTelemetry(),
	}

	timestamp := file.Now()
	captureName := "capture-rotating"
	nodeHostName := "node-host-name"
	filter := ""
	maxSize := 50
	fileCount := 10
	os.Setenv(captureConstants.CaptureNameEnvKey, captureName)
	os.Setenv(captureConstants.NodeHostNameEnvKey, nodeHostName)
	os.Setenv(captureConstants.CaptureStartTimestampEnvKey, file.TimeToString(timestamp))
	os.Setenv(captureConstants.CaptureDurationEnvKey, "3600s")
	os.Setenv(captureConstants.CaptureMaxSizeEnvKey, strconv.Itoa(maxSize))
	os.Setenv(captureConstants.CaptureFileCountEnvKey, strconv.Itoa(fileCount))

	defer func() {
		os.Unsetenv(captureConstants.CaptureNameEnvKey)
		os.Unsetenv(captureConstants.NodeHostNameEnvKey)
		os.Unsetenv(captureConstants.CaptureStartTimestampEnvKey)
		os.Unsetenv(captureConstants.TcpdumpFilterEnvKey)
		os.Unsetenv(captureConstants.CaptureDurationEnvKey)
		os.Unsetenv(captureConstants.CaptureMaxSizeEnvKey)
		os.Unsetenv(captureConstants.CaptureFileCountEnvKey)
	}()

	ctx, cancel := TestContext(t)
	defer cancel()

	tmpFilename := file.CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: timestamp}
	networkCaptureProvider.EXPECT().Setup(tmpFilename).Return(fmt.Sprintf("%s-%s-%s", captureName, nodeHostName, timestamp), nil).Times(1)
	networkCaptureProvider.EXPECT().CaptureNetworkPacket(ctx, filter, 3600, maxSize, fileCount).Return(nil).Times(1)

	_, err := cm.CaptureNetwork(ctx)
	if err != nil {
		t.Errorf("CaptureNetwork with rotating capture should have not fail with error %s", err)
	}
}

func TestCaptureNetworkWithNoDuration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	networkCaptureProvider := provider.NewMockNetworkCaptureProviderInterface(ctrl)
	cm := &CaptureManager{
		networkCaptureProvider: networkCaptureProvider,
		tel:                    telemetry.NewNoopTelemetry(),
	}

	timestamp := file.Now()
	captureName := "capture-no-duration"
	nodeHostName := "node-host-name"
	maxSize := 100
	fileCount := 5
	os.Setenv(captureConstants.CaptureNameEnvKey, captureName)
	os.Setenv(captureConstants.NodeHostNameEnvKey, nodeHostName)
	os.Setenv(captureConstants.CaptureStartTimestampEnvKey, file.TimeToString(timestamp))
	// No CAPTURE_DURATION set - simulating rotating capture without duration
	os.Setenv(captureConstants.CaptureMaxSizeEnvKey, strconv.Itoa(maxSize))
	os.Setenv(captureConstants.CaptureFileCountEnvKey, strconv.Itoa(fileCount))

	defer func() {
		os.Unsetenv(captureConstants.CaptureNameEnvKey)
		os.Unsetenv(captureConstants.NodeHostNameEnvKey)
		os.Unsetenv(captureConstants.CaptureStartTimestampEnvKey)
		os.Unsetenv(captureConstants.TcpdumpFilterEnvKey)
		os.Unsetenv(captureConstants.CaptureDurationEnvKey)
		os.Unsetenv(captureConstants.CaptureMaxSizeEnvKey)
		os.Unsetenv(captureConstants.CaptureFileCountEnvKey)
	}()

	ctx, cancel := TestContext(t)
	defer cancel()

	tmpFilename := file.CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: timestamp}
	networkCaptureProvider.EXPECT().Setup(tmpFilename).Return(fmt.Sprintf("%s-%s-%s", captureName, nodeHostName, timestamp), nil).Times(1)
	// duration=0 when env is not set, fileCount=5
	networkCaptureProvider.EXPECT().CaptureNetworkPacket(ctx, "", 0, maxSize, fileCount).Return(nil).Times(1)

	_, err := cm.CaptureNetwork(ctx)
	if err != nil {
		t.Errorf("CaptureNetwork with no duration should have not fail with error %s", err)
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

func TestCaptureDuration(t *testing.T) {
	cm := &CaptureManager{}

	tests := []struct {
		name     string
		envValue string
		want     int
		wantErr  bool
	}{
		{name: "empty string returns 0", envValue: "", want: 0, wantErr: false},
		{name: "valid duration 10s", envValue: "10s", want: 10, wantErr: false},
		{name: "valid duration 1h", envValue: "1h", want: 3600, wantErr: false},
		{name: "invalid duration", envValue: "notaduration", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(captureConstants.CaptureDurationEnvKey, tt.envValue)
			defer os.Unsetenv(captureConstants.CaptureDurationEnvKey)

			got, err := cm.captureDuration()
			if (err != nil) != tt.wantErr {
				t.Errorf("captureDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("captureDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCaptureFileCount(t *testing.T) {
	cm := &CaptureManager{}

	tests := []struct {
		name     string
		envValue string
		want     int
		wantErr  bool
	}{
		{name: "empty string returns 0", envValue: "", want: 0, wantErr: false},
		{name: "valid count", envValue: "5", want: 5, wantErr: false},
		{name: "invalid value", envValue: "abc", want: 0, wantErr: true},
		{name: "negative value rejected", envValue: "-1", want: 0, wantErr: true},
		{name: "zero is valid", envValue: "0", want: 0, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(captureConstants.CaptureFileCountEnvKey, tt.envValue)
			defer os.Unsetenv(captureConstants.CaptureFileCountEnvKey)

			got, err := cm.captureFileCount()
			if (err != nil) != tt.wantErr {
				t.Errorf("captureFileCount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("captureFileCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompressFolderToTarGzIncludesRotatedFiles(t *testing.T) {
	// Simulate rotating capture output: tcpdump with -C and -W creates
	// files like capture.pcap0, capture.pcap1, capture.pcap2, etc.
	srcDir := t.TempDir()

	rotatedFiles := []string{
		"capture.pcap0",
		"capture.pcap1",
		"capture.pcap2",
		"capture.pcap3",
	}

	for i, name := range rotatedFiles {
		content := fmt.Sprintf("pcap-data-file-%d", i)
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
	}

	dstFile := filepath.Join(t.TempDir(), "output.tar.gz")
	if err := compressFolderToTarGz(srcDir, dstFile); err != nil {
		t.Fatalf("compressFolderToTarGz failed: %v", err)
	}

	// Extract and verify all rotated files are present
	archivedFiles := extractTarGzFileNames(t, dstFile)
	sort.Strings(archivedFiles)
	sort.Strings(rotatedFiles)

	if diff := cmp.Diff(rotatedFiles, archivedFiles); diff != "" {
		t.Errorf("archived files mismatch (-want +got):\n%s", diff)
	}
}

func TestCompressFolderToTarGzPreservesContent(t *testing.T) {
	srcDir := t.TempDir()

	files := map[string]string{
		"capture.pcap0": "data-for-file-0",
		"capture.pcap1": "data-for-file-1",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
	}

	dstFile := filepath.Join(t.TempDir(), "output.tar.gz")
	if err := compressFolderToTarGz(srcDir, dstFile); err != nil {
		t.Fatalf("compressFolderToTarGz failed: %v", err)
	}

	// Extract and verify content
	extracted := extractTarGzContents(t, dstFile)
	for name, wantContent := range files {
		got, ok := extracted[name]
		if !ok {
			t.Errorf("file %s not found in archive", name)
			continue
		}
		if got != wantContent {
			t.Errorf("file %s: got content %q, want %q", name, got, wantContent)
		}
	}
}

// extractTarGzFileNames returns the names of regular files in a tar.gz archive.
func extractTarGzFileNames(t *testing.T, archivePath string) []string {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			names = append(names, hdr.Name)
		}
	}
	return names
}

// extractTarGzContents returns a map of filename -> content for regular files.
func extractTarGzContents(t *testing.T, archivePath string) map[string]string {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	contents := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("failed to read file %s: %v", hdr.Name, err)
			}
			contents[hdr.Name] = string(data)
		}
	}
	return contents
}
