//go:build localtest

// Copyright 2022 The Inspektor Gadget authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//nolint:typecheck
package main

import (
	"context"
	"time"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/dns"
	"github.com/microsoft/retina/pkg/pubsub"
	"go.uber.org/zap"
)

func main() {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)
	l := log.Logger().Named("test-dns")

	metrics.InitializeMetrics()
	ctx := context.Background()

	tt := dns.New(&config.Config{
		EnablePodLevel: true,
	})

	err := tt.Stop()
	if err != nil {
		l.Error("Failed to stop dns plugin", zap.Error(err))
		return
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	c := cache.New(pubsub.New())
	e := enricher.New(ctx, c)
	e.Run()

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
		l.Error("Failed to start dns plugin", zap.Error(err))
		return
	}
	l.Info("Started dns")

	defer func() {
		if err := tt.Stop(); err != nil {
			l.Error("Failed to stop dns plugin", zap.Error(err))
		}
	}()

	for range ctxTimeout.Done() {
	}
}
