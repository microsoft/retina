// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package common contains common functions and types used by all Retina plugins.
package common

import (
	"fmt"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// FileSystemChecker allows mocking Statfs for testing.
type FileSystemChecker interface {
	Statfs(path string, buf *unix.Statfs_t) error
}

// UnixFileSystemChecker performs real syscalls.
type UnixFileSystemChecker struct{}

func (u *UnixFileSystemChecker) Statfs(path string, buf *unix.Statfs_t) error {
	if err := unix.Statfs(path, buf); err != nil {
		return fmt.Errorf("Statfs failed for %s: %w", path, err)
	}
	return nil
}

type fsInfo struct {
	name     string
	paths    []string
	magic    int64
	required bool
}

// CheckMountedFilesystems checks required kernel filesystems.
func CheckMountedFilesystems(l *log.ZapLogger) error {
	return CheckMountedFilesystemsWithChecker(l, &UnixFileSystemChecker{})
}

// CheckMountedFilesystemsWithChecker is testable version.
func CheckMountedFilesystemsWithChecker(l *log.ZapLogger, checker FileSystemChecker) error {
	filesystems := []fsInfo{
		{
			name:     "bpf",
			paths:    []string{"/sys/fs/bpf"},
			magic:    unix.BPF_FS_MAGIC,
			required: false, // bpffs is less critical
		},
		{
			name:     "debugfs",
			paths:    []string{"/sys/kernel/debug"},
			magic:    unix.DEBUGFS_MAGIC,
			required: true,
		},
		{
			name:     "tracefs",
			paths:    []string{"/sys/kernel/tracing", "/sys/kernel/debug/tracing"},
			magic:    unix.TRACEFS_MAGIC,
			required: true,
		},
	}

	missingRequired := false

	for _, fs := range filesystems {
		found := false

		for _, path := range fs.paths {
			var stat unix.Statfs_t
			err := checker.Statfs(path, &stat)
			if err != nil {
				l.Debug("filesystem check error", zap.String("fs", fs.name), zap.String("path", path), zap.Error(err))
				continue
			}

			if stat.Type == fs.magic {
				l.Debug("filesystem mounted", zap.String("fs", fs.name), zap.String("path", path))
				found = true
				break
			}
		}

		if !found {
			l.Error("filesystem NOT mounted", zap.String("fs", fs.name), zap.Strings("paths", fs.paths))
			if fs.required {
				missingRequired = true
			}
		}
	}

	if missingRequired {
		return fmt.Errorf("required filesystem missing: %w", unix.ENOENT)
	}
	return nil
}
