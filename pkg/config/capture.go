// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package config

import (
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
)

// CaptureConfig defines the configuration for capture controller in the operator.
type CaptureConfig struct {
	// Configurations to determine the capture workload image.
	//
	// Debug indicates whether to enable debug mode.
	// If true, the operator will pick the image from the test container registry for the capture workload.
	// Check pkg/capture/utils/capture_image.go for the detailed explanation of how debug capture image version is picked.
	// NOTE: CaptureImageVersion and CaptureImageVersionSource are used internally and not visible to the user.
	CaptureDebug bool `yaml:"captureDebug"`
	// ImageVersion defines the image version of the capture workload.
	CaptureImageVersion string `yaml:"-"`
	// VersionSource defines the source of the image version.
	CaptureImageVersionSource captureUtils.VersionSource `yaml:"-"`

	// JobNumLimit indicates the maximum number of jobs that can be created for each Capture.
	CaptureJobNumLimit int `yaml:"captureJobNumLimit"`

	// EnableManagedStorageAccount indicates whether a managed storage account will be created to store the captured network artifacts.
	EnableManagedStorageAccount bool `yaml:"enableManagedStorageAccount"`
	// AzureCredentialConfig indicates the path of Azure credential configuration file.
	AzureCredentialConfig string `yaml:"azureCredentialConfig"`
}
