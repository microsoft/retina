// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package watchermanager

import (
	"context"
	"fmt"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/watchers/apiserver"
	"github.com/microsoft/retina/pkg/watchers/veth"
	"go.uber.org/zap"
)

const (
	// DefaultRefreshRate is the default refresh rate for watchers.
	DefaultRefreshRate = 30 * time.Second
)

func NewWatcherManager() *WatcherManager {
	return &WatcherManager{
		Watchers: []IWatcher{
			apiserver.Watcher(),
		},
		l:           log.Logger().Named("watcher-manager"),
		refreshRate: DefaultRefreshRate,
	}
}

func (wm *WatcherManager) Start(ctx context.Context) error {
	newCtx, cancelCtx := context.WithCancel(ctx)
	wm.cancel = cancelCtx

	vethWatcher := veth.NewWatcher()
	go vethWatcher.Start(newCtx)

	for _, w := range wm.Watchers {
		if err := w.Init(ctx); err != nil {
			wm.l.Error("init failed", zap.String("watcher_type", fmt.Sprintf("%T", w)))
			return err
		}
		wm.wg.Add(1)
		go wm.runWatcher(newCtx, w)
		wm.l.Info("watcher started", zap.String("watcher_type", fmt.Sprintf("%T", w)))
	}
	return nil
}

func (wm *WatcherManager) Stop(ctx context.Context) error {
	if wm.cancel != nil {
		wm.cancel() // cancel all runWatcher
	}
	for _, w := range wm.Watchers {
		if err := w.Stop(ctx); err != nil {
			wm.l.Error("failed to stop", zap.String("watcher_type", fmt.Sprintf("%T", w)), zap.Error(err))
			return err
		}
	}
	wm.wg.Wait() // wait for all runWatcher to stop
	wm.l.Info("watcher manager stopped")
	return nil
}

func (wm *WatcherManager) runWatcher(ctx context.Context, w IWatcher) error {
	defer wm.wg.Done() // signal that this runWatcher is done
	ticker := time.NewTicker(wm.refreshRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			wm.l.Info("watcher stopping...", zap.String("watcher_type", fmt.Sprintf("%T", w)))
			return nil
		case <-ticker.C:
			err := w.Refresh(ctx)
			if err != nil {
				wm.l.Error("refresh failed", zap.Error(err))
				return err
			}
		}
	}
}
