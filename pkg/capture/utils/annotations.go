// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
)

func GetPodAnnotationsFromCapture(capture *retinav1alpha1.Capture) map[string]string {
	return map[string]string{
		captureConstants.CaptureFilenameAnnotationKey:  capture.Name,
		captureConstants.CaptureTimestampAnnotationKey: file.TimeToString(capture.Status.StartTime),
		captureConstants.CaptureHostPathAnnotationKey:  *capture.Spec.OutputConfiguration.HostPath,
	}
}
