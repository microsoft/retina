//go:build localtest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package main

import (
	"context"
	"net"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/managers/filtermanager"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/dropreason"

	"go.uber.org/zap"
)

func main() {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)
	l := log.Logger().Named("test-dropreason")

	metrics.InitializeMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  true,
	}

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

	// Add IPs to filtermanager.
	// ipsToAdd := []string{"10.224.0.106", "10.224.0.101"}
	// ipsToAdd := []string{"20.69.116.85", "10.224.0.6"}
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

	tt := dropreason.New(cfg)

	err = tt.Stop()
	if err != nil {
		l.Error("Stop failed:%v", zap.Error(err))
		return
	}

	err = tt.Generate(ctx)
	if err != nil {
		l.Error("Generate failed:%v", zap.Error(err))
		return
	}

	err = tt.Compile(ctx)
	if err != nil {
		l.Error("Compile failed:%v", zap.Error(err))
		return
	}

	err = tt.Init()
	if err != nil {
		l.Error("Init failed:%v", zap.Error(err))
		return
	}

	err = tt.Start(ctx)
	if err != nil {
		l.Error("Start failed:%v", zap.Error(err))
		return
	}
	l.Info("Started dropreason")

	defer tt.Stop()
	for range ctx.Done() {
	}
}
