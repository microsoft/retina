// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package main

import (
	"context"
	"net"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/managers/watchermanager"
	"github.com/microsoft/retina/pkg/plugin/packetparser"
	"github.com/microsoft/retina/pkg/watchers/endpoint"
	"go.uber.org/zap"

	"github.com/microsoft/retina/pkg/metrics"
)

func main() {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)
	l := log.Logger().Named("test-packetparser")

	metrics.InitializeMetrics()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	// watcher manager
	wm := watchermanager.NewWatcherManager()
	wm.Watchers = []watchermanager.IWatcher{endpoint.Watcher()}

	err := wm.Start(ctxTimeout)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := wm.Stop(ctxTimeout); err != nil {
			l.Error("Stop endpoint watcher failed", zap.Error(err))
		}
	}()
	// Filtermanager.
	f, err := filtermanager.Init(3)
	if err != nil {
		l.Error("Start filtermanager failed", zap.Error(err))
		return
	}
	defer func() {
		if err := f.Stop(); err != nil {
			l.Error("Stop filtermanager failed", zap.Error(err))
		}
	}()

	ipsToAdd := []string{"20.69.116.85"}
	ips := []net.IP{}
	for _, ip := range ipsToAdd {
		x := net.ParseIP(ip).To4()
		if x == nil {
			l.Fatal("Invalid IP address", zap.String("ip", ip))
		}
		ips = append(ips, x)
	}
	err = f.AddIPs(ips, "packetparser-test", filtermanager.RequestMetadata{RuleID: "test"})
	if err != nil {
		l.Error("AddIPs failed", zap.Error(err))
		return
	}

	// Start packetparser plugin.
	cfg := &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  true,
	}
	p := packetparser.New(cfg)
	if err = p.Stop(); err != nil {
		l.Error("Stop packetparser plugin failed", zap.Error(err))
		return
	}

	defer cancel()
	err = p.Generate(ctxTimeout)
	if err != nil {
		l.Error("Generate failed", zap.Error(err))
		return
	}

	err = p.Compile(ctxTimeout)
	if err != nil {
		l.Error("Compile failed", zap.Error(err))
		return
	}

	err = p.Init()
	if err != nil {
		l.Error("Init failed", zap.Error(err))
		return
	}

	err = p.Start(ctxTimeout)
	if err != nil {
		l.Error("Start failed", zap.Error(err))
		return
	}
	l.Info("Started packetparser")

	for range ctxTimeout.Done() {
		l.Info("packetparser is running")
		time.Sleep(1 * time.Second)
	}

	err = p.Stop()
	if err != nil {
		l.Error("Stop failed", zap.Error(err))
		return
	}
	l.Info("Stopping packetparser")
}
