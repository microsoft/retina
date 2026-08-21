// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows/registry"
)

var errRegistryFailure = errors.New("registry failure")

// fakeRegistryKey is an in-memory stand-in for registry.Key.
type fakeRegistryKey struct {
	value    uint64
	valueErr error
	closed   bool
}

func (f *fakeRegistryKey) GetIntegerValue(string) (uint64, uint32, error) {
	if f.valueErr != nil {
		return 0, 0, f.valueErr
	}
	return f.value, registry.DWORD, nil
}

func (f *fakeRegistryKey) Close() error {
	f.closed = true
	return nil
}

// TestIsCiliumOnWindowsEnabled covers the full registry matrix that gates the mutually
// exclusive startup of the ebpfwindows and hnsstats plugins.
func TestIsCiliumOnWindowsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		key      *fakeRegistryKey
		openErr  error
		expected bool
		wantErr  error
	}{
		{
			name:    "registry key missing means cilium is disabled",
			openErr: registry.ErrNotExist,
		},
		{
			name: "registry value missing means cilium is disabled",
			key:  &fakeRegistryKey{valueErr: registry.ErrNotExist},
		},
		{
			name: "registry value 0 means cilium is disabled",
			key:  &fakeRegistryKey{value: 0},
		},
		{
			name:     "registry value 1 means cilium is enabled",
			key:      &fakeRegistryKey{value: 1},
			expected: true,
		},
		{
			name: "unexpected registry value means cilium is disabled",
			key:  &fakeRegistryKey{value: 2},
		},
		{
			name:    "failure opening the key is surfaced",
			openErr: errRegistryFailure,
			wantErr: errRegistryFailure,
		},
		{
			name:    "failure reading the value is surfaced",
			key:     &fakeRegistryKey{valueErr: errRegistryFailure},
			wantErr: errRegistryFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := openRegistryKey
			t.Cleanup(func() { openRegistryKey = original })

			var openedPath string
			openRegistryKey = func(path string) (registryKey, error) {
				openedPath = path
				if tt.openErr != nil {
					return nil, tt.openErr
				}
				return tt.key, nil
			}

			enabled, err := IsCiliumOnWindowsEnabled()

			require.Equal(t, tt.expected, enabled)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, KeyPath, openedPath)
			if tt.key != nil {
				require.True(t, tt.key.closed, "registry key should be closed")
			}
		})
	}
}
