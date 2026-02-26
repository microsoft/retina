//go:build linux
// +build linux

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package packetparser

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRingBufferSize(t *testing.T) {
	const maxSize = 1 * 1024 * 1024 * 1024 // 1GB
	intPageSize := os.Getpagesize()
	if intPageSize <= 0 {
		intPageSize = 4096
	}
	if intPageSize > int(^uint32(0)) {
		intPageSize = int(^uint32(0))
	}
	//nolint:gosec // bounded to uint32
	pageSize := uint32(intPageSize)

	tests := []struct {
		name        string
		inputSize   uint32
		expectedErr bool
		expectedMsg string
	}{
		{
			name:        "Zero input returns error",
			inputSize:   0,
			expectedErr: true,
			expectedMsg: "must be set",
		},
		{
			name:        "Below page size returns error",
			inputSize:   pageSize - 1,
			expectedErr: true,
			expectedMsg: "page size",
		},
		{
			name:        "Above max size returns error",
			inputSize:   maxSize + 1,
			expectedErr: true,
			expectedMsg: "maximum",
		},
		{
			name:        "Not power of 2 returns error",
			inputSize:   (8 * 1024 * 1024) + 1,
			expectedErr: true,
			expectedMsg: "power of 2",
		},
		{
			name:        "Valid size returns no error",
			inputSize:   16 * 1024 * 1024,
			expectedErr: false,
		},
		{
			name:        "Valid max size returns no error",
			inputSize:   maxSize,
			expectedErr: false,
		},
		{
			name:        "Valid page size returns no error (assuming page size is power of 2)",
			inputSize:   pageSize,
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRingBufferSize(tt.inputSize)
			if tt.expectedErr {
				require.Error(t, err)
				if tt.expectedMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
