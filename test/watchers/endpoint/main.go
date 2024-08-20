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
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func main() {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	_, err := log.SetupZapLogger(opts)
	if err != nil {
		panic(err)
	}
	l := log.Logger().Named("test-endpoint-watcher")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// watcher manager
	wm := watchermanager.NewWatcherManager()
	wm.Watchers = []watchermanager.Watcher{endpoint.NewWatcher()}

	g, ctx := errgroup.WithContext(ctx)
	// Start watcher manager
	g.Go(func() error {
		err = wm.Start(ctx)
		if err != nil {
			l.Error("watcher manager exited with error", zap.Error(err))
			return errors.Wrap(err, "watcher manager exited with error")
		}
		return nil
	})

	// Sleep 1 minute.
	time.Sleep(60 * time.Second) // nolint:gomnd // Sleep is used for testing purposes.

	err = wm.Stop(ctx)
	if err != nil {
		l.Error("Failed to start watcher manager", zap.Error(err))
		panic(err)
	}
}
