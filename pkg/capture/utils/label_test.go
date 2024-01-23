// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
)

func TestGetSerectLabelsFromCaptureName(t *testing.T) {
	captureName := "test"
	labels := GetSerectLabelsFromCaptureName(captureName)
	assert.Equal(t, labels[label.AppLabel], captureConstants.CaptureAppname)
	assert.Equal(t, labels[label.CaptureNameLabel], captureName)
}

func TestGetJobLabelsFromCaptureName(t *testing.T) {
	captureName := "test"
	labels := GetJobLabelsFromCaptureName(captureName)
	assert.Equal(t, labels[label.AppLabel], captureConstants.CaptureAppname)
	assert.Equal(t, labels[label.CaptureNameLabel], captureName)
}

func TestGetContainerLabelsFromCaptureName(t *testing.T) {
	captureName := "test"
	labels := GetContainerLabelsFromCaptureName(captureName)
	assert.Equal(t, labels[label.AppLabel], captureConstants.CaptureAppname)
	assert.Equal(t, labels[label.CaptureNameLabel], captureName)
}
