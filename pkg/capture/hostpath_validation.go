// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// DefaultHostPathAllowedPrefix is the safe default location under which Capture CRs may
// place captured artifacts on a node when no operator-level allowlist is configured.
const DefaultHostPathAllowedPrefix = "/var/log/retina/captures"

var (
	// ErrHostPathEmpty is returned when the supplied HostPath is empty.
	ErrHostPathEmpty = errors.New("hostPath is empty")
	// ErrHostPathNotAbsolute is returned when the supplied HostPath is not an absolute path.
	ErrHostPathNotAbsolute = errors.New("hostPath must be an absolute path")
	// ErrHostPathTraversal is returned when the supplied HostPath contains a parent-directory traversal.
	ErrHostPathTraversal = errors.New("hostPath must not contain '..' path segments")
	// ErrHostPathNotAllowed is returned when the supplied HostPath is not under any configured allowed prefix.
	ErrHostPathNotAllowed = errors.New("hostPath is not under an allowed prefix")
)

// validateHostPath ensures that the user-supplied HostPath from a Capture CR is safe to
// mount into the privileged capture pod. It returns the cleaned path on success.
//
// Rules:
//   - The path must be non-empty.
//   - The path must be absolute.
//   - The path must not contain any ".." segment (rejected both pre- and post-clean).
//   - After cleaning, the path must be exactly one of, or nested under, an entry in
//     allowedPrefixes. Comparison uses a trailing separator to prevent prefix-confusion
//     (e.g. "/foo" must not match "/foo-evil").
//
// If allowedPrefixes is empty, DefaultHostPathAllowedPrefix is used.
func validateHostPath(raw string, allowedPrefixes []string) (string, error) {
	if raw == "" {
		return "", ErrHostPathEmpty
	}

	// Reject literal ".." segments before cleaning so traversal attempts are rejected
	// even if filepath.Clean would normalize them away.
	for _, seg := range strings.Split(filepath.ToSlash(raw), "/") {
		if seg == ".." {
			return "", fmt.Errorf("%w: %q", ErrHostPathTraversal, raw)
		}
	}

	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("%w: %q", ErrHostPathNotAbsolute, raw)
	}

	cleaned := filepath.Clean(raw)

	// Defense in depth: re-check after cleaning.
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("%w: %q", ErrHostPathNotAbsolute, raw)
	}
	for _, seg := range strings.Split(filepath.ToSlash(cleaned), "/") {
		if seg == ".." {
			return "", fmt.Errorf("%w: %q", ErrHostPathTraversal, raw)
		}
	}

	prefixes := allowedPrefixes
	if len(prefixes) == 0 {
		prefixes = []string{DefaultHostPathAllowedPrefix}
	}

	sep := string(filepath.Separator)
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		cp := filepath.Clean(p)
		if cleaned == cp || strings.HasPrefix(cleaned, cp+sep) {
			return cleaned, nil
		}
	}

	return "", fmt.Errorf("%w: %q (allowed prefixes: %v)", ErrHostPathNotAllowed, raw, prefixes)
}
