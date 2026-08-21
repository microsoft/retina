// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package common contains common functions and types used by all Retina Windows plugins.
package common

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const (
	// KeyPath is the registry key path where the CiliumOnWindows value is stored.
	// This key is used to determine if Cilium is enabled on Windows.
	KeyPath = `SYSTEM\CurrentControlSet\Services\hns\State`
	// CiliumOnWindows is the registry value name that indicates if Cilium is enabled on Windows.
	// If this value is set to 1, Cilium is enabled on Windows. If this value is not set or set to 0, Cilium is not enabled.
	ValueName = "CiliumOnWindows"
)

// registryKey is the subset of registry.Key used to read the CiliumOnWindows value.
// It exists so tests can inject a fake registry.
type registryKey interface {
	GetIntegerValue(name string) (val uint64, valtype uint32, err error)
	Close() error
}

// openRegistryKey opens the registry key holding the CiliumOnWindows value.
var openRegistryKey = func(path string) (registryKey, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return nil, err //nolint:wrapcheck // the caller distinguishes registry.ErrNotExist and wraps
	}
	return k, nil
}

// IsCiliumOnWindowsEnabled checks if the CiliumOnWindows registry value is set to 1.
// Returns (true, nil) if set to 1, (false, nil) if not set or not exist, (false, err) for other errors.
func IsCiliumOnWindowsEnabled() (bool, error) {
	k, err := openRegistryKey(KeyPath)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("opening registry key: %w", err)
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue(ValueName)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("reading registry value: %w", err)
	}
	return val == 1, nil
}
