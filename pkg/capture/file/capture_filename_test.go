// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package file

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CaptureFilenameFormat(t *testing.T) {
	captureName := "capture-name"
	nodeHostName := "node1"
	timestamp := v1.NewTime(time.Date(2025, 7, 16, 12, 30, 0, 0, time.UTC))
	name := CaptureFilename{CaptureName: captureName, NodeHostname: nodeHostName, StartTimestamp: &timestamp}
	assert.Equal(t, name.String(), "capture-name-node1-20250716123000UTC")
}
