// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package bpf

import (
	"fmt"
	"os"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/constants"
	"github.com/microsoft/retina/pkg/plugin/filter"
	"github.com/cilium/cilium/pkg/mountinfo"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func __mount() error {
	// Check if the path exists.
	_, err := os.Stat(constants.FilterMapPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Path does not exist. Create it.
			err = os.MkdirAll(constants.FilterMapPath, 0o755)
			if err != nil {
				return err
			}
		} else {
			// Some other error. Return.
			return err
		}
	}
	err = unix.Mount(constants.FilterMapPath, constants.FilterMapPath, "bpf", 0, "")
	return err
}

func mountBpfFs() error {
	// Check if /sys/fs/bpf is already mounted.
	mounted, bpfMount, err := mountinfo.IsMountFS(mountinfo.FilesystemTypeBPFFS, constants.FilterMapPath)
	if err != nil {
		return err
	}
	if !mounted {
		if err := __mount(); err != nil {
			return err
		}
		return nil
	}
	// Else mounted. Check the type of mount.
	if !bpfMount {
		// Custom mount of /sys/fs/bpf. Unknown setup. Exit.
		return fmt.Errorf("%+s is already mounted but not as bpf. Not supported", constants.FilterMapPath)
	}
	return nil
}

func Setup(l *log.ZapLogger) {
	err := mountBpfFs()
	if err != nil {
		l.Panic("Failed to mount bpf filesystem", zap.Error(err))
	}
	l.Info("BPF filesystem mounted successfully", zap.String("path", constants.FilterMapPath))

	// Delete existing filter map file.
	err = os.Remove(constants.FilterMapPath + "/" + constants.FilterMapName)
	if err != nil && !os.IsNotExist(err) {
		l.Panic("Failed to delete existing filter map file", zap.Error(err))
	}
	l.Info("Deleted existing filter map file", zap.String("path", constants.FilterMapPath), zap.String("Map name", constants.FilterMapName))

	// Initialize the filter map.
	// This will create the filter map in kernel and pin it to /sys/fs/bpf.
	_, err = filter.Init()
	if err != nil {
		l.Panic("Failed to initialize filter map", zap.Error(err))
	}
	l.Info("Filter map initialized successfully", zap.String("path", constants.FilterMapPath), zap.String("Map name", constants.FilterMapName))
}
