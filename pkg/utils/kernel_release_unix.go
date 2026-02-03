//go:build unix

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// KernelRelease returns the kernel release string (e.g. "5.15.0-101-generic").
func KernelRelease() (string, error) {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return "", fmt.Errorf("uname failed: %w", err)
	}
	return charsToString(uts.Release[:]), nil
}

func charsToString(ca []byte) string {
	n := 0
	for ; n < len(ca); n++ {
		if ca[n] == 0 {
			break
		}
	}
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		b[i] = ca[i]
	}
	return string(b)
}
