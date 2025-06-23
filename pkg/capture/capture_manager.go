// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"go.uber.org/zap"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	captureOutput "github.com/microsoft/retina/pkg/capture/outputlocation"
	captureProvider "github.com/microsoft/retina/pkg/capture/provider"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CaptureManager captures network packets and metadata into tar ball, then send the tar ball to the location(s)
// specified by users.
type CaptureManager struct {
	l                      *log.ZapLogger
	networkCaptureProvider captureProvider.NetworkCaptureProviderInterface
	tel                    telemetry.Telemetry
}

func NewCaptureManager(logger *log.ZapLogger, tel telemetry.Telemetry) *CaptureManager {
	return &CaptureManager{
		l:                      logger,
		networkCaptureProvider: captureProvider.NewNetworkCaptureProvider(logger),
		tel:                    tel,
	}
}

func (cm *CaptureManager) CaptureNetwork(ctx context.Context) (string, error) {
	startTimestamp, err := cm.captureStartTimestamp()
	if err != nil {
		return "", err
	}

	filename := file.CaptureFilename{CaptureName: cm.captureName(), NodeHostname: cm.captureNodeHostName(), StartTimestamp: startTimestamp}
	tmpLocation, err := cm.networkCaptureProvider.Setup(filename)
	if err != nil {
		return "", err
	}

	captureFilter := cm.captureFilter()

	captureDuration, err := cm.captureDuration()
	if err != nil {
		return "", err
	}

	captureMaxSizeMB, err := cm.captureMaxSizeMB()
	if err != nil {
		return "", err
	}

	if err := cm.networkCaptureProvider.CaptureNetworkPacket(ctx, captureFilter, captureDuration, captureMaxSizeMB); err != nil {
		return "", err
	}

	if includeMetadata := cm.includeMetadata(); includeMetadata {
		if err := cm.networkCaptureProvider.CollectMetadata(); err != nil {
			return "", err
		}
	}

	// no-op telemetry client will not send any telemetry
	cm.tel.TrackEvent("capturenetwork", map[string]string{
		"captureName": cm.captureName(),
		"nodeName":    cm.captureNodeHostName(),
		"filter":      captureFilter,
		"duration":    strconv.Itoa(captureDuration),
		"maxSizeMB":   strconv.Itoa(captureMaxSizeMB),
	})

	return tmpLocation, nil
}

func (cm *CaptureManager) Cleanup() error {
	if err := cm.networkCaptureProvider.Cleanup(); err != nil {
		cm.l.Error("Failed to cleanup capture job", zap.String("capture name", cm.captureName()), zap.Error(err))
		return err
	}

	return nil
}

func (cm *CaptureManager) captureName() string {
	return os.Getenv(captureConstants.CaptureNameEnvKey)
}

func (cm *CaptureManager) captureNodeHostName() string {
	return os.Getenv(captureConstants.NodeHostNameEnvKey)
}

func (cm *CaptureManager) captureStartTimestamp() (*metav1.Time, error) {
	timestamp, err := file.StringToTime((os.Getenv(captureConstants.CaptureStartTimestampEnvKey)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	return timestamp, nil
}

func (cm *CaptureManager) captureFilter() string {
	captureFilter := os.Getenv(captureConstants.TcpdumpFilterEnvKey)
	if runtime.GOOS == "windows" {
		captureFilter = os.Getenv(captureConstants.NetshFilterEnvKey)
	}
	return captureFilter
}

func (cm *CaptureManager) captureDuration() (int, error) {
	captureDurationStr := os.Getenv(captureConstants.CaptureDurationEnvKey)
	duration, err := time.ParseDuration(captureDurationStr)
	if err != nil {
		return 0, err
	}
	return int(duration.Seconds()), nil
}

func (cm *CaptureManager) includeMetadata() bool {
	includeMetadataStr := os.Getenv(captureConstants.IncludeMetadataEnvKey)
	if len(includeMetadataStr) == 0 {
		return false
	}

	// As strconv.ParseBool returns false when hitting error, considering we enable includemetadata by default,
	// we return false and log the error.
	includeMetadata, err := strconv.ParseBool(includeMetadataStr)
	if err != nil {
		cm.l.Error("Failed to parse string to boolean", zap.String("includeMetadata", includeMetadataStr), zap.Error(err))
	}

	return includeMetadata
}

func (cm *CaptureManager) captureMaxSizeMB() (int, error) {
	captureMaxSizeMBStr := os.Getenv(captureConstants.CaptureMaxSizeEnvKey)
	if len(captureMaxSizeMBStr) == 0 {
		return 0, nil
	}
	return strconv.Atoi(captureMaxSizeMBStr)
}

func (cm *CaptureManager) OutputCapture(ctx context.Context, srcDir string) error {
	var errs error

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("capture source directory %s does not exist", srcDir)
	}

	dstTarGz := srcDir + ".tar.gz"
	if err := compressFolderToTarGz(srcDir, dstTarGz); err != nil {
		return err
	}

	for _, location := range cm.enabledOutputLocations() {
		if err := location.Output(ctx, dstTarGz); err != nil {
			errs = fmt.Errorf("%w; location %q output error: %w", errs, location.Name(), err)
		}
	}

	if errs != nil {
		return fmt.Errorf("failed to enable output locations: %w", errs)
	}

	// Remove tarball created inside this function.
	if err := os.Remove(dstTarGz); err != nil {
		cm.l.Error("Failed to delete tarball", zap.String("tarball name", dstTarGz), zap.Error(err))
	}

	return nil
}

func (cm *CaptureManager) enabledOutputLocations() []captureOutput.Location {
	locations := []captureOutput.Location{}
	if hostPath := captureOutput.NewHostPath(cm.l); hostPath.Enabled() {
		locations = append(locations, hostPath)
	}
	if bu := captureOutput.NewBlobUpload(cm.l); bu.Enabled() {
		locations = append(locations, bu)
	}
	if pvc := captureOutput.NewPersistentVolumeClaim(cm.l); pvc.Enabled() {
		locations = append(locations, pvc)
	}
	if s3 := captureOutput.NewS3Upload(cm.l); s3.Enabled() {
		locations = append(locations, s3)
	}
	return locations
}

func compressFolderToTarGz(src string, dst string) error {
	// Create the output file
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// Create the gzip writer
	gz := gzip.NewWriter(out)
	defer gz.Close()

	// Create the tar writer
	tarWriter := tar.NewWriter(gz)
	defer tarWriter.Close()

	// Walk the source directory and add files to the tar archive
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create a new tar header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// Set the header name to the relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write the header and file contents to the tar archive
		if err = tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err = io.Copy(tarWriter, file); err != nil {
			return err
		}

		return nil
	})

	return err
}
