// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package ebpfwindows

import (
    "context"
    "testing"
    "time"

    kcfg "github.com/microsoft/retina/pkg/config"
    "github.com/microsoft/retina/pkg/log"
    "go.uber.org/zap"
)

func TestPlugin(t *testing.T) {
    log.SetupZapLogger(log.GetDefaultLogOpts())
    l := log.Logger().Named("test-ebpf")

    ctx := context.Background()

    cfg := &kcfg.Config{
        MetricsInterval: 1 * time.Second,
        EnablePodLevel:  true,
    }

    tt := New(cfg)

    err := tt.Stop()
    if err != nil {
        l.Error("Failed to stop windows ebpf plugin", zap.Error(err))
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
        l.Error("Failed to start windows ebpf plugin", zap.Error(err))
        return
    }
    l.Info("Started windows ebpf plugin")

    defer func() {
        if err := tt.Stop(); err != nil {
            l.Error("Failed to stop windows ebpf plugin", zap.Error(err))
        }
    }()

    for range ctx.Done() {
    }
}
