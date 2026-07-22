// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultHostPathBaseDir is the safe default location under which Capture CRs may
// place captured artifacts on a node when no operator-level base directory is configured.
const DefaultHostPathBaseDir = "/var/log/retina/captures"

var (
	// ErrHostPathEmpty is returned when the supplied HostPath is empty.
	ErrHostPathEmpty = errors.New("hostPath is empty")
	// ErrHostPathAbsolute is returned when the supplied HostPath is absolute. The
	// CR field is a subpath name only; it must not contain a leading separator
	// (POSIX or Windows) or a drive-letter prefix.
	ErrHostPathAbsolute = errors.New("hostPath must be a relative subpath name, not an absolute path")
	// ErrHostPathTraversal is returned when the supplied HostPath contains a parent-directory traversal.
	ErrHostPathTraversal = errors.New("hostPath must not contain '..' path segments")
	// ErrHostPathEscapesBase is a defense-in-depth error returned when, after
	// joining and cleaning, the resulting path would lie outside the configured base directory.
	ErrHostPathEscapesBase = errors.New("hostPath resolves outside the configured base directory")
	// ErrHostPathBaseDir is returned when the operator-provided base directory is not usable.
	ErrHostPathBaseDir = errors.New("invalid hostPath base directory")
)

// winDriveLetter matches a Windows drive-letter prefix such as "C:\" or "c:/".
var winDriveLetter = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// validateHostPath ensures that the user-supplied HostPath from a Capture CR is safe
// to mount into the privileged capture pod and returns the absolute, cleaned path the
// capture artifacts will live at on the node.
//
// The CR's HostPath is treated as a relative subpath name and joined under baseDir;
// CR authors cannot escape that directory. Rules:
//
//   - The path must be non-empty.
//   - The path must not be absolute (no leading "/" or "\\", no Windows drive letter).
//   - The path must not contain any ".." segment, checked both on the raw input and
//     after filepath.Clean.
//   - As defense in depth, the joined path must still resolve under baseDir.
//
// If baseDir is empty, DefaultHostPathBaseDir is used.
func validateHostPath(raw, baseDir string) (string, error) {
	if raw == "" {
		return "", ErrHostPathEmpty
	}

	if baseDir == "" {
		baseDir = DefaultHostPathBaseDir
	}
	cleanedBase := filepath.Clean(baseDir)
	if !filepath.IsAbs(cleanedBase) {
		return "", fmt.Errorf("%w: %q must be absolute", ErrHostPathBaseDir, baseDir)
	}

	// Reject absolute paths up front, in both POSIX and Windows styles, so existing
	// CRs that supplied an absolute host path fail loudly instead of being silently
	// rewritten by filepath.Join.
	if filepath.IsAbs(raw) ||
		strings.HasPrefix(raw, "/") ||
		strings.HasPrefix(raw, `\`) ||
		winDriveLetter.MatchString(raw) {
		return "", fmt.Errorf("%w: %q", ErrHostPathAbsolute, raw)
	}

	// Reject literal ".." segments before cleaning so traversal attempts are
	// rejected even if filepath.Clean would normalize them away.
	if containsParentSegment(raw) {
		return "", fmt.Errorf("%w: %q", ErrHostPathTraversal, raw)
	}

	cleanedSub := filepath.Clean(raw)
	if cleanedSub == "." || cleanedSub == "" {
		return "", ErrHostPathEmpty
	}
	if filepath.IsAbs(cleanedSub) || strings.HasPrefix(cleanedSub, "/") || strings.HasPrefix(cleanedSub, `\`) {
		return "", fmt.Errorf("%w: %q", ErrHostPathAbsolute, raw)
	}
	if containsParentSegment(cleanedSub) {
		return "", fmt.Errorf("%w: %q", ErrHostPathTraversal, raw)
	}

	joined := filepath.Clean(filepath.Join(cleanedBase, cleanedSub))
	rel, err := filepath.Rel(cleanedBase, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q resolves to %q (base %q)", ErrHostPathEscapesBase, raw, joined, cleanedBase)
	}

	return joined, nil
}

// containsParentSegment returns true if any path segment of p (split on either
// forward or back slashes) equals "..".
func containsParentSegment(p string) bool {
	for _, seg := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return true
		}
	}
	return false
}
