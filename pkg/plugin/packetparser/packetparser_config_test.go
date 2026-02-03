//go:build linux
// +build linux

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package packetparser

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRingBufferSize(t *testing.T) {
	const defaultSize = 8 * 1024 * 1024
	const maxSize = 1 * 1024 * 1024 * 1024 // 1GB
	intPageSize := os.Getpagesize()
	if intPageSize <= 0 {
		intPageSize = 4096
	}
	if intPageSize > int(^uint32(0)) {
		intPageSize = int(^uint32(0))
	}
	pageSize := uint32(intPageSize)

	tests := []struct {
		name           string
		inputSize      uint32
		expectedSize   uint32
		expectedReason string
	}{
		{
			name:           "Zero input returns default",
			inputSize:      0,
			expectedSize:   defaultSize,
			expectedReason: "",
		},
		{
			name:           "Below page size returns default",
			inputSize:      pageSize - 1,
			expectedSize:   defaultSize,
			expectedReason: "Ring buffer size", // partial match
		},
		{
			name:           "Above max size returns default",
			inputSize:      maxSize + 1,
			expectedSize:   defaultSize,
			expectedReason: "Ring buffer size", // partial match
		},
		{
			name:           "Not power of 2 returns default",
			inputSize:      defaultSize + 1,
			expectedSize:   defaultSize,
			expectedReason: "Ring buffer size", // partial match
		},
		{
			name:           "Valid size returns same size",
			inputSize:      16 * 1024 * 1024,
			expectedSize:   16 * 1024 * 1024,
			expectedReason: "",
		},
		{
			name:           "Valid max size returns same size",
			inputSize:      maxSize,
			expectedSize:   maxSize,
			expectedReason: "",
		},
		{
			name:           "Valid page size returns same size (assuming page size is power of 2)",
			inputSize:      pageSize,
			expectedSize:   pageSize,
			expectedReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, reason := validateRingBufferSize(tt.inputSize)
			assert.Equal(t, tt.expectedSize, size)
			if tt.expectedReason != "" {
				assert.Contains(t, reason, tt.expectedReason)
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}
