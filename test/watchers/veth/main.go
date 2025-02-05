// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package main

import (
	"context"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/watchermanager"
	"github.com/microsoft/retina/pkg/watchers/endpoint"
	"go.uber.org/zap"
)

func main() {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)
	l := log.Logger().Named("test-veth")

	ctx := context.Background()

	// watcher manager
	wm := watchermanager.NewWatcherManager()
	wm.Watchers = []watchermanager.Watcher{endpoint.NewWatcher()}

	err := wm.Start(ctx)
	if err != nil {
		panic(err)
	}

	// Sleep 1 minute.
	time.Sleep(60 * time.Second)

	err = wm.Stop(ctx)
	if err != nil {
		l.Error("Failed to start watcher manager", zap.Error(err))
		panic(err)
	}
}
