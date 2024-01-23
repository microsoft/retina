// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
)

func GetSerectLabelsFromCaptureName(captureName string) map[string]string {
	return map[string]string{
		label.AppLabel:         captureConstants.CaptureAppname,
		label.CaptureNameLabel: captureName,
	}
}

func GetJobLabelsFromCaptureName(captureName string) map[string]string {
	return map[string]string{
		label.AppLabel:         captureConstants.CaptureAppname,
		label.CaptureNameLabel: captureName,
	}
}

func GetContainerLabelsFromCaptureName(captureName string) map[string]string {
	return map[string]string{
		label.AppLabel:         captureConstants.CaptureAppname,
		label.CaptureNameLabel: captureName,
	}
}
