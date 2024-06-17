// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package main

import (
	"context"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/infiniband"
	"go.uber.org/zap"
)

func main() {
	log.SetupZapLogger(log.GetDefaultLogOpts()) //nolint std.
	l := log.Logger().Named("test-infiniband")

	metrics.InitializeMetrics()

	cfg := &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  true,
	}
	tt := infiniband.New(cfg)
	err := tt.Init()
	if err != nil {
		l.Error("Init failed:%v", zap.Error(err))
		return
	}
	ctx := context.Background()
	err = tt.Start(ctx)
	if err != nil {
		l.Error("start failed:%v", zap.Error(err))
		return
	}
	l.Info("started infiniband logger")

	defer func() {
		err := tt.Stop()
		if err != nil {
			l.Error("stop failed:%v", zap.Error(err))
		}
	}()

	<-ctx.Done()
}
