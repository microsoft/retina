// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
)

// GetPodAnnotationsFromCapture builds the capture pod annotations. resolvedHostPath,
// when non-empty, is written as the CaptureHostPathAnnotationKey value (the on-node
// path actually mounted into the capture pod). When empty, the raw value from the
// Capture spec is used as a fallback (e.g. when called from contexts where the
// host path has not been resolved yet).
func GetPodAnnotationsFromCapture(capture *retinav1alpha1.Capture, resolvedHostPath string) map[string]string {
	annotations := map[string]string{
		captureConstants.CaptureFilenameAnnotationKey: capture.Name,
	}
	if capture.Status.StartTime != nil {
		annotations[captureConstants.CaptureTimestampAnnotationKey] = file.TimeToString(capture.Status.StartTime)
	}
	if resolvedHostPath != "" {
		annotations[captureConstants.CaptureHostPathAnnotationKey] = resolvedHostPath
	} else if capture.Spec.OutputConfiguration.HostPath != nil {
		annotations[captureConstants.CaptureHostPathAnnotationKey] = *capture.Spec.OutputConfiguration.HostPath
	}
	return annotations
}
