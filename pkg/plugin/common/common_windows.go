// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package common contains common functions and types used by all Retina Windows plugins.
package common

import (
	"golang.org/x/sys/windows/registry"
)

// IsCiliumOnWindowsEnabled checks if the CiliumOnWindows registry value is set to 1.
// Returns (true, nil) if set to 1, (false, nil) if not set or not exist, (false, err) for other errors.
func IsCiliumOnWindowsEnabled() (bool, error) {
	keyPath := `SYSTEM\CurrentControlSet\Services\hns\State`
	valueName := "CiliumOnWindows"

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue(valueName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	return val == 1, nil
}
