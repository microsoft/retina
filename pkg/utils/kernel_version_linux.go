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

	"golang.org/x/mod/semver"
)

// KernelVersion represents a parsed Linux kernel version.
type KernelVersion struct {
	Major   int
	Minor   int
	Patch   int
	Release string
}

var (
	errUnexpectedKernelReleaseFormat = errors.New("unexpected kernel release format")
	errInvalidKernelSemver           = errors.New("invalid kernel semver")
)

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
	current := kernelSemverFromParts(v.Major, v.Minor, v.Patch)
	required := kernelSemverFromParts(major, minor, patch)
	return semver.Compare(current, required) >= 0
}

// ParseLinuxKernelRelease parses the uname release string into semantic version parts.
func ParseLinuxKernelRelease(release string) (major, minor, patch int, err error) {
	_, major, minor, patch, err = normalizeKernelReleaseToSemver(release)
	if err != nil {
		return 0, 0, 0, err
	}
	return major, minor, patch, nil
}

func kernelSemverFromParts(major, minor, patch int) string {
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch)
}

func normalizeKernelReleaseToSemver(release string) (version string, major, minor, patch int, err error) {
	base := strings.SplitN(release, "-", 2)[0]
	parts := strings.Split(base, ".")
	if len(parts) < 2 {
		return "", 0, 0, 0, fmt.Errorf("%w: %q", errUnexpectedKernelReleaseFormat, release)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("invalid kernel major in %q: %w", release, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("invalid kernel minor in %q: %w", release, err)
	}
	patch = 0
	if len(parts) >= 3 {
		if patch, err = strconv.Atoi(parts[2]); err != nil {
			return "", 0, 0, 0, fmt.Errorf("invalid kernel patch in %q: %w", release, err)
		}
	}

	version = kernelSemverFromParts(major, minor, patch)
	if !semver.IsValid(version) {
		return "", 0, 0, 0, fmt.Errorf("%w for %q: %s", errInvalidKernelSemver, release, version)
	}

	return version, major, minor, patch, nil
}
