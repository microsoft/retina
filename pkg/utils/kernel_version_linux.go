//go:build linux

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//revive:disable:var-naming
package utils

//revive:enable:var-naming

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// KernelVersion represents a parsed Linux kernel version.
type KernelVersion struct {
	Major   int
	Minor   int
	Patch   int
	Release string
}

var errUnexpectedKernelReleaseFormat = errors.New("unexpected kernel release format")

// LinuxKernelVersion returns the parsed Linux kernel version and release string.
func LinuxKernelVersion() (KernelVersion, error) {
	release, err := KernelRelease()
	if err != nil {
		return KernelVersion{}, err
	}
	major, minor, patch, err := ParseLinuxKernelRelease(release)
	if err != nil {
		return KernelVersion{Release: release}, err
	}
	return KernelVersion{Major: major, Minor: minor, Patch: patch, Release: release}, nil
}

// AtLeast reports whether the kernel version is >= the required version.
func (v KernelVersion) AtLeast(major, minor, patch int) bool {
	if v.Major != major {
		return v.Major > major
	}
	if v.Minor != minor {
		return v.Minor > minor
	}
	return v.Patch >= patch
}

// ParseLinuxKernelRelease parses the uname release string into semantic version parts.
func ParseLinuxKernelRelease(release string) (major, minor, patch int, err error) {
	base := strings.SplitN(release, "-", 2)[0]
	parts := strings.Split(base, ".")
	if len(parts) < 2 {
		return 0, 0, 0, fmt.Errorf("%w: %q", errUnexpectedKernelReleaseFormat, release)
	}

	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid kernel major in %q: %w", release, err)
	}
	if minor, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid kernel minor in %q: %w", release, err)
	}
	patch = 0
	if len(parts) >= 3 {
		if patch, err = strconv.Atoi(parts[2]); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid kernel patch in %q: %w", release, err)
		}
	}

	return major, minor, patch, nil
}
