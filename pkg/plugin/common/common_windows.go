// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package common contains common functions and types used by all Retina Windows plugins.
package common

import (
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

// IsCiliumOnWindowsEnabled checks if the CiliumOnWindows registry value is set to 1.
// Returns (true, nil) if set to 1, (false, nil) if not set or not exist, (false, err) for other errors.
func IsCiliumOnWindowsEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, KeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue(ValueName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	return val == 1, nil
}
