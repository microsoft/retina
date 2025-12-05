// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build linux
// +build linux

package common

import (
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// CheckAndMountFilesystems checks if required filesystems are mounted.
// Returns an error if any required filesystem is not available.
// This helps prevent os.Exit() calls from dependencies that expect these filesystems.
func CheckAndMountFilesystems(l *log.ZapLogger) error {
	filesystems := []struct {
		name     string
		paths    []string
		magic    int64
		required bool // if true, return error if not available
	}{
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

	var firstError error
filesystemLoop:
	for _, fs := range filesystems {
		var statfs unix.Statfs_t
		for _, path := range fs.paths {
			if err := unix.Statfs(path, &statfs); err != nil {
				l.Debug("statfs returned error", zap.String("fs", fs.name), zap.String("path", path), zap.Error(err))
				continue
			}
			if statfs.Type == fs.magic {
				l.Debug("Filesystem already mounted", zap.String("fs", fs.name), zap.String("path", path))
				continue filesystemLoop
			}
		}

		// Filesystem not found or not mounted
		l.Error("Filesystem not mounted", zap.String("fs", fs.name), zap.Strings("paths", fs.paths))
		if fs.required && firstError == nil {
			firstError = unix.ENOENT
		}
	}
	return firstError
}
