// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package bpf

import (
	"os"

	"github.com/cilium/cilium/pkg/mountinfo"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/filter"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func mount() error {
	// Check if the path exists.
	_, err := os.Stat(plugincommon.MapPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Path does not exist. Create it.
			err = os.MkdirAll(plugincommon.MapPath, 0o755) //nolint:gomnd // 0o755 is the permission.
			if err != nil {
				return errors.Wrap(err, "failed to create bpf filesystem path")
			}
		} else {
			// Some other error. Return.
			return errors.Wrap(err, "failed to stat bpf filesystem path")
		}
	}
	err = unix.Mount(plugincommon.MapPath, plugincommon.MapPath, "bpf", 0, "")
	return errors.Wrap(err, "failed to mount bpf filesystem")
}

func mountBpfFs() error {
	// Check if /sys/fs/bpf is already mounted.
	mounted, bpfMount, err := mountinfo.IsMountFS(mountinfo.FilesystemTypeBPFFS, plugincommon.MapPath)
	if err != nil {
		return errors.Wrap(err, "failed to check if bpf filesystem is mounted")
	}
	if !mounted {
		if err := mount(); err != nil {
			return err
		}
		return nil
	}
	// Else mounted. Check the type of mount.
	if !bpfMount {
		// Custom mount of /sys/fs/bpf. Unknown setup. Exit.
		return errors.New("bpf filesystem is mounted but not as bpf type")
	}
	return nil
}

func Setup(l *zap.Logger) {
	err := mountBpfFs()
	if err != nil {
		l.Panic("Failed to mount bpf filesystem", zap.Error(err))
	}
	l.Info("BPF filesystem mounted successfully", zap.String("path", plugincommon.MapPath))

	// Delete existing filter map file.
	err = os.Remove(plugincommon.MapPath + "/" + plugincommon.FilterMapName)
	if err != nil && !os.IsNotExist(err) {
		l.Panic("Failed to delete existing filter map file", zap.Error(err))
	}
	l.Info("Deleted existing filter map file", zap.String("path", plugincommon.MapPath), zap.String("Map name", plugincommon.FilterMapName))

	// Initialize the filter map.
	// This will create the filter map in kernel and pin it to /sys/fs/bpf.
	_, err = filter.Init()
	if err != nil {
		l.Panic("Failed to initialize filter map", zap.Error(err))
	}
	l.Info("Filter map initialized successfully", zap.String("path", plugincommon.MapPath), zap.String("Map name", plugincommon.FilterMapName))
}
