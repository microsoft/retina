// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package main

import (
	"os"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/bpf"
	"github.com/microsoft/retina/pkg/log"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/conntrack"
	"github.com/microsoft/retina/pkg/plugin/filter"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
)

const ciliumDir = "/var/run/cilium"

func main() {
	// Initialize logger
	opts := log.GetDefaultLogOpts()

	// Enable telemetry if applicationInsightsID is provided
	if buildinfo.ApplicationInsightsID != "" {
		opts.EnableTelemetry = true
		opts.ApplicationInsightsID = buildinfo.ApplicationInsightsID
		// Initialize application insights
		telemetry.InitAppInsights(buildinfo.ApplicationInsightsID, buildinfo.Version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()
	}

	zl, err := log.SetupZapLogger(opts)
	if err != nil {
		panic(err)
	}
	l := zl.Named("init-retina").With(zap.String("version", buildinfo.Version))

	// Setup Retina BPF filesystem.
	setupRetinaBpfFs(l)

	// Setup CiliumFS.
	setupCiliumFs(l)
}

func setupRetinaBpfFs(l *zap.Logger) {
	err := bpf.MountRetinaBpfFS()
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

func setupCiliumFs(l *zap.Logger) {
	// Create /var/run/cilium directory.
	fp, err := os.Stat(ciliumDir)
	if err != nil {
		l.Warn("Failed to stat directory", zap.String("dir path", ciliumDir), zap.Error(err))
		if os.IsNotExist(err) {
			l.Info("Directory does not exist", zap.String("dir path", ciliumDir), zap.Error(err))
			// Path does not exist. Create it.
			err = os.MkdirAll("/var/run/cilium", 0o755) //nolint:gomnd // 0o755 is the permission mode.
			if err != nil {
				l.Error("Failed to create directory", zap.String("dir path", ciliumDir), zap.Error(err))
				l.Panic("Failed to create directory", zap.String("dir path", ciliumDir), zap.Error(err))
			}
		} else {
			// Some other error. Return.
			l.Panic("Failed to stat directory", zap.String("dir path", ciliumDir), zap.Error(err))
		}
	}
	l.Info("Created directory", zap.String("dir path", ciliumDir), zap.Any("file", fp))
}
