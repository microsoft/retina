// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package constants

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

	// CaptureWorkloadImageName defines the official capture workload image repo and image name
	CaptureWorkloadImageName string = "mcr.microsoft.com/containernetworking/retina-agent"

	// DebugCaptureWorkloadImageName defines the capture workload image for testing and debugging
	DebugCaptureWorkloadImageName string = "acnpublic.azurecr.io/retina-agent"
)
