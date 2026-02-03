//go:build linux

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//revive:disable-next-line:var-naming
package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLinuxKernelRelease(t *testing.T) {
	tests := []struct {
		name        string
		release     string
		expectMajor int
		expectMinor int
		expectPatch int
		expectErr   bool
	}{
		{
			name:        "full version with suffix",
			release:     "5.15.0-101-generic",
			expectMajor: 5,
			expectMinor: 15,
			expectPatch: 0,
			expectErr:   false,
		},
		{
			name:        "no patch version",
			release:     "6.1-foo",
			expectMajor: 6,
			expectMinor: 1,
			expectPatch: 0,
			expectErr:   false,
		},
		{
			name:        "full numeric version",
			release:     "4.19.260",
			expectMajor: 4,
			expectMinor: 19,
			expectPatch: 260,
			expectErr:   false,
		},
		{
			name:      "invalid format",
			release:   "foo",
			expectErr: true,
		},
		{
			name:      "invalid minor",
			release:   "5.x.1",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch, err := ParseLinuxKernelRelease(tt.release)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectMajor, major)
			assert.Equal(t, tt.expectMinor, minor)
			assert.Equal(t, tt.expectPatch, patch)
		})
	}
}

func TestKernelVersionAtLeast(t *testing.T) {
	v := KernelVersion{Major: 5, Minor: 8, Patch: 0}

	assert.True(t, v.AtLeast(5, 8, 0))
	assert.True(t, v.AtLeast(5, 7, 99))
	assert.False(t, v.AtLeast(5, 8, 1))
	assert.False(t, v.AtLeast(6, 0, 0))

	v2 := KernelVersion{Major: 6, Minor: 1, Patch: 3}
	assert.True(t, v2.AtLeast(5, 8, 0))
	assert.True(t, v2.AtLeast(6, 1, 3))
	assert.False(t, v2.AtLeast(6, 2, 0))
}
