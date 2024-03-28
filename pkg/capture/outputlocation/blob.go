// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"go.uber.org/zap"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

type BlobUpload struct {
	l *log.ZapLogger
}

var _ Location = &BlobUpload{}

func NewBlobUpload(logger *log.ZapLogger) Location {
	return &BlobUpload{l: logger}
}

func (bu *BlobUpload) Name() string {
	return "BlobUpload"
}

func (bu *BlobUpload) Enabled() bool {
	_, err := readBlobURL()
	if err != nil {
		bu.l.Debug("Output location is not enabled", zap.String("location", bu.Name()))
		return false
	}
	return true
}

func (bu *BlobUpload) Output(srcFilePath string) error {
	bu.l.Info("Upload capture file to blob.", zap.String("location", bu.Name()))
	blobURL, err := readBlobURL()
	if err != nil {
		bu.l.Error("Failed to read blob url", zap.Error(err))
		return err
	}

	if err = validateBlobURL(blobURL); err != nil {
		bu.l.Error("Failed to validate blob url", zap.Error(err))
		return err
	}

	// TODO: add retry policy
	azClient, err := azblob.NewClientWithNoCredential(blobURL, nil)
	if err != nil {
		bu.l.Error("Failed to create blob client", zap.String("location", bu.Name()), zap.Error(err))
		return err
	}

	blobFile, err := os.Open(srcFilePath)
	if err != nil {
		bu.l.Error("Failed to open capture file", zap.Error(err))
		return err
	}
	defer blobFile.Close()

	blobName := filepath.Base(srcFilePath)
	_, err = azClient.UploadFile(
		context.TODO(),
		"",
		blobName,
		blobFile,
		// TODO: add metadata
		&azblob.UploadFileOptions{})
	if err != nil {
		bu.l.Error("Failed to upload file to storage account", zap.String("location", bu.Name()), zap.Error(err))
		return err
	}
	bu.l.Info("Done for uploading capture file to storage account", zap.String("location", bu.Name()))
	return nil
}

func readBlobURL() (string, error) {
	secretPath := filepath.Join(captureConstants.CaptureOutputLocationBlobUploadSecretPath, captureConstants.CaptureOutputLocationBlobUploadSecretKey)
	if runtime.GOOS == "windows" {
		containerSandboxMountPoint := os.Getenv(captureConstants.ContainerSandboxMountPointEnvKey)
		if len(containerSandboxMountPoint) == 0 {
			return "", fmt.Errorf("failed to find sandbox mount path through env %s", captureConstants.ContainerSandboxMountPointEnvKey)
		}
		secretPath = filepath.Join(containerSandboxMountPoint, captureConstants.CaptureOutputLocationBlobUploadSecretPath, captureConstants.CaptureOutputLocationBlobUploadSecretKey)
	}
	secretBytes, err := os.ReadFile(secretPath)
	return string(secretBytes), err
}

func validateBlobURL(blobURL string) error {
	u, err := url.Parse(blobURL)
	if err != nil {
		return err
	}

	// Split the path into storage account container and blob
	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		return fmt.Errorf("invalid blob URL")
	}

	return nil
}
