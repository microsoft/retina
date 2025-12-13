// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common //nolint:revive // ignore var-naming for clarity

import (
	"errors"
	"testing"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sys/unix"
)

type fakeFSChecker struct {
	// map[path] => (magic, error)
	results map[string]struct {
		magic int64
		err   error
	}
}

var (
	ErrPathNotFound = errors.New("path not found in fakeFSChecker results")
	ErrNotMounted   = errors.New("not mounted")
	ErrNoBpf        = errors.New("no bpf")
)

func (f *fakeFSChecker) Statfs(path string, buf *unix.Statfs_t) error {
	if r, ok := f.results[path]; ok {
		if r.err != nil {
			return r.err
		}
		buf.Type = r.magic
		return nil
	}
	return ErrPathNotFound
}

type ZapLogger struct {
	*zap.SugaredLogger
}

func TestCheckMountedFilesystems(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)

	logger := &log.ZapLogger{
		Logger: zapLogger,
	}

	tests := []struct {
		name    string
		results map[string]struct {
			magic int64
			err   error
		}
		expectErr bool
	}{
		{
			name: "all required filesystems mounted",
			results: map[string]struct {
				magic int64
				err   error
			}{
				"/sys/kernel/debug":   {unix.DEBUGFS_MAGIC, nil},
				"/sys/kernel/tracing": {unix.TRACEFS_MAGIC, nil},
				"/sys/fs/bpf":         {unix.BPF_FS_MAGIC, nil},
			},
			expectErr: false,
		},
		{
			name: "missing required debugfs",
			results: map[string]struct {
				magic int64
				err   error
			}{
				"/sys/kernel/tracing": {unix.TRACEFS_MAGIC, nil},
			},
			expectErr: true,
		},
		{
			name: "tracefs mounted via second path",
			results: map[string]struct {
				magic int64
				err   error
			}{
				"/sys/kernel/debug":         {unix.DEBUGFS_MAGIC, nil},
				"/sys/kernel/tracing":       {0, ErrNotMounted},
				"/sys/kernel/debug/tracing": {unix.TRACEFS_MAGIC, nil}, // fallback works
			},
			expectErr: false,
		},
		{
			name: "non-required bpf missing but required ones present",
			results: map[string]struct {
				magic int64
				err   error
			}{
				"/sys/kernel/debug":   {unix.DEBUGFS_MAGIC, nil},
				"/sys/kernel/tracing": {unix.TRACEFS_MAGIC, nil},
			},
			expectErr: false,
		},
		{
			name: "both required filesystems missing",
			results: map[string]struct {
				magic int64
				err   error
			}{
				"/sys/fs/bpf": {0, ErrNoBpf},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &fakeFSChecker{
				results: tt.results,
			}
			err := CheckMountedFilesystemsWithChecker(logger, checker)
			if tt.expectErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
