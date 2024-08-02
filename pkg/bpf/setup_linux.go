// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package bpf

import (
	"fmt"
	"os"

	"github.com/cilium/cilium/pkg/mountinfo"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/conntrack"
	"github.com/microsoft/retina/pkg/plugin/filter"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func __mount() error {
	// Check if the path exists.
	_, err := os.Stat(plugincommon.MapPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Path does not exist. Create it.
			err = os.MkdirAll(plugincommon.MapPath, 0o755)
			if err != nil {
				return err
			}
		} else {
			// Some other error. Return.
			return err
		}
	}
	err = unix.Mount(plugincommon.MapPath, plugincommon.MapPath, "bpf", 0, "")
	return err
}

func mountBpfFs() error {
	// Check if /sys/fs/bpf is already mounted.
	mounted, bpfMount, err := mountinfo.IsMountFS(mountinfo.FilesystemTypeBPFFS, plugincommon.MapPath)
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
		return fmt.Errorf("%+s is already mounted but not as bpf. Not supported", plugincommon.MapPath)
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

	// Delete existing conntrack map file.
	err = os.Remove(plugincommon.MapPath + "/" + plugincommon.ConntrackMapName)
	if err != nil && !os.IsNotExist(err) {
		l.Panic("Failed to delete existing conntrack map file", zap.Error(err))
	}
	l.Info("Deleted existing conntrack map file", zap.String("path", plugincommon.MapPath), zap.String("Map name", plugincommon.ConntrackMapName))

	// Initialize the filter map.
	// This will create the filter map in kernel and pin it to /sys/fs/bpf.
	_, err = filter.Init()
	if err != nil {
		l.Panic("Failed to initialize filter map", zap.Error(err))
	}
	l.Info("Filter map initialized successfully", zap.String("path", plugincommon.MapPath), zap.String("Map name", plugincommon.FilterMapName))

	// Initialize the conntrack map.
	// This will create the conntrack map in kernel and pin it to /sys/fs/bpf.
	ct := conntrack.New(nil)
	err = ct.Init()
	if err != nil {
		l.Panic("Failed to initialize conntrack map", zap.Error(err))
	}
	l.Info("Conntrack map initialized successfully", zap.String("path", plugincommon.MapPath))
}
