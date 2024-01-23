// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package main

import (
	"context"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/managers/watchermanager"
	"github.com/microsoft/retina/pkg/watchers/apiserver"
	"go.uber.org/zap"
)

func main() {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)
	l := log.Logger().Named("test-apiserver")

	ctx := context.Background()

	// Filtermanager.
	f, err := filtermanager.Init(5)
	if err != nil {
		l.Error("Failed to start Filtermanager", zap.Error(err))
		panic(err)
	}
	defer func() {
		if err := f.Stop(); err != nil {
			l.Error("Failed to stop Filtermanager", zap.Error(err))
		}
	}()
	// watcher manager
	wm := watchermanager.NewWatcherManager()
	wm.Watchers = []watchermanager.IWatcher{apiserver.Watcher()}

	// apiserver watcher.
	err = wm.Start(ctx)
	if err != nil {
		l.Error("Failed to start watcher manager", zap.Error(err))
		panic(err)
	}

	// Sleep 1 minute.
	time.Sleep(60 * time.Second)

	err = wm.Stop(ctx)
	if err != nil {
		panic(err)
	}
}
