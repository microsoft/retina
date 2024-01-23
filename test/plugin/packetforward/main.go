// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package main

import (
	"context"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/packetforward"

	kcfg "github.com/microsoft/retina/pkg/config"

	"github.com/microsoft/retina/pkg/metrics"
	"go.uber.org/zap"
)

func main() {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test-packetforward")

	metrics.InitializeMetrics()

	ctx := context.Background()

	cfg := &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  true,
	}
	tt := packetforward.New(cfg)

	err := tt.Stop()
	if err != nil {
		l.Error("Failed to stop packetforward plugin", zap.Error(err))
		return
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	err = tt.Generate(ctxTimeout)
	if err != nil {
		l.Error("Failed to generate the plugin specific header files", zap.Error(err))
		return
	}

	err = tt.Compile(ctxTimeout)
	if err != nil {
		l.Error("Failed to compile the ebpf to generate bpf object", zap.Error(err))
		return
	}

	err = tt.Init()
	if err != nil {
		l.Error("Failed to initialize plugin specific objects", zap.Error(err))
		return
	}

	err = tt.Start(ctx)
	if err != nil {
		l.Error("Failed to start packetforward plugin", zap.Error(err))
		return
	}
	l.Info("Started packetforward")

	defer func() {
		if err := tt.Stop(); err != nil {
			l.Error("Failed to stop packetforward plugin", zap.Error(err))
		}
	}()

	for range ctx.Done() {
	}
}
