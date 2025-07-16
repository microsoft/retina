// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package file

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCaptureFilenameFormat(t *testing.T) {
	tests := []struct {
		name         string
		captureName  string
		nodeHostname string
		timestamp    *v1.Time
		expected     string
	}{
		{
			name:         "valid filename",
			captureName:  "capture-name",
			nodeHostname: "node1",
			timestamp:    &v1.Time{Time: time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)},
			expected:     "capture-name-node1-20250101123000UTC",
		},
		{
			name:         "different timezone",
			captureName:  "capture-name",
			nodeHostname: "node1",
			timestamp:    &v1.Time{Time: time.Date(2025, 1, 1, 8, 30, 0, 0, time.FixedZone("EDT", -4*60*60))}, // 8:30 EDT is 12:30 UTC
			expected:     "capture-name-node1-20250101123000UTC",
		},
		{
			name:         "empty capture name",
			captureName:  "",
			nodeHostname: "node1",
			timestamp:    &v1.Time{Time: time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)},
			expected:     "-node1-20250101123000UTC",
		},
		{
			name:         "empty node name",
			captureName:  "capture-name",
			nodeHostname: "",
			timestamp:    &v1.Time{Time: time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)},
			expected:     "capture-name--20250101123000UTC",
		},
		{
			name:         "zero time",
			captureName:  "capture-name",
			nodeHostname: "node1",
			timestamp:    &v1.Time{Time: time.Time{}},
			expected:     "capture-name-node1-00010101000000UTC",
		},
		{
			name:         "nil timestamp",
			captureName:  "capture-name",
			nodeHostname: "node1",
			timestamp:    nil,
			expected:     "capture-name-node1-00010101000000UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := CaptureFilename{
				CaptureName:    tt.captureName,
				NodeHostname:   tt.nodeHostname,
				StartTimestamp: tt.timestamp,
			}

			// .String() here relies on TimeToString(), which should handle nil timestamps gracefully
			// and return a zero time string - this should not panic
			var result string
			assert.NotPanics(t, func() {
				result = filename.String()
			}, "CaptureFilename.String() should handle nil timestamp gracefully")

			assert.Equal(t, tt.expected, result)
		})
	}
}
