// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package watchermanager

import (
	"context"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/watchers/apiserver"
	"github.com/microsoft/retina/pkg/watchers/veth"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func NewWatcherManager() *WatcherManager {
	return &WatcherManager{
		Watchers: []Watcher{
			apiserver.NewWatcher(),
			veth.NewWatcher(),
		},
		l: log.Logger().Named("watcher-manager"),
	}
}

func (wm *WatcherManager) Start(ctx context.Context) error {
	wm.l.Info("starting watcher manager")
	// start all watchers
	g, ctx := errgroup.WithContext(ctx)
	for _, w := range wm.Watchers {
		w := w
		g.Go(func() error {
			wm.l.Info("starting watcher", zap.String("name", w.Name()))
			return w.Start(ctx)
		})
	}
	err := g.Wait()
	if err != nil {
		wm.l.Error("watcher manager exited with error", zap.Error(err))
		return errors.Wrap(err, "watcher manager exited with error")
	}
	return nil
}

func (wm *WatcherManager) Stop(ctx context.Context) error {
	wm.l.Info("watcher manager stopped")
	return nil
}
