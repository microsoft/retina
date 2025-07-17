// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package constants

import (
	"github.com/microsoft/retina/internal/buildinfo"
)

// Capture job specification
const (
	CaptureHostPathVolumeName string = "hostpath"
	CapturePVCVolumeName      string = "pvc"

	// PersistentVolumeClaimVolumeMountPathLinux is the PVC volume mount path of container hosted on Linux node.
	PersistentVolumeClaimVolumeMountPathLinux string = "/mnt/azure"
	// PersistentVolumeClaimVolumeMountPathWin is the PVC volume mount path of container hosted on Windows node.
	PersistentVolumeClaimVolumeMountPathWin string = "D:"

	CaptureContainerEntrypoint    string = "./retina/captureworkload"
	CaptureContainerEntrypointWin string = "captureworkload.exe"

	CaptureAppname       string = "capture"
	CaptureContainername string = "capture"

	// CaptureOutputLocationBlobUploadSecretName is the name of the secret that stores the blob upload url.
	CaptureOutputLocationBlobUploadSecretName string = "capture-blob-upload-secret"
	// CaptureOutputLocationBlobUploadSecretPath is the path of the secret that stores the blob upload url.
	CaptureOutputLocationBlobUploadSecretPath string = "/etc/blob-upload-secret"
	// CaptureOutputLocationBlobUploadSecretKey is the key of the secret that stores the blob upload url.
	CaptureOutputLocationBlobUploadSecretKey string = "blob-upload-url"

	// CaptureOutputLocationS3UploadSecretName is the name of the secret that stores the s3 credentials.
	CaptureOutputLocationS3UploadSecretName string = "capture-s3-upload-secret" // #nosec G101
	// CaptureOutputLocationS3UploadSecretPath is the path of the secret that stores the s3 credentials.
	CaptureOutputLocationS3UploadSecretPath string = "/etc/s3-upload-secret" // #nosec G101
	// CaptureOutputLocationS3UploadAccessKeyID is the key of the secret that stores the s3 access key id.
	CaptureOutputLocationS3UploadAccessKeyID string = "s3-access-key-id"
	// CaptureOutputLocationS3UploadSecretAccessKey is the key of the secret that stores the s3 secret access key.
	CaptureOutputLocationS3UploadSecretAccessKey string = "s3-secret-access-key"

	// DebugCaptureWorkloadImageName defines the capture workload image for testing and debugging
	DebugCaptureWorkloadImageName string = "ghcr.io/microsoft/retina/retina-agent"
)

// CaptureWorkloadImageName defines the official capture workload image repo and image name
var CaptureWorkloadImageName string = buildinfo.RetinaAgentImageName
