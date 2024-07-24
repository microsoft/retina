// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpoint

import (
	"context"
	"time"

	"github.com/microsoft/retina/pkg/common"
	"go.uber.org/zap"
)

func (w *Watcher) Name() string {
	return watcherName
}

func (w *Watcher) Start(ctx context.Context) error {
	w.l.Info("Starting endpoint watcher")
	ticker := time.NewTicker(w.refreshRate)
	for {
		select {
		case <-ctx.Done():
			w.l.Info("context done, stopping endpoint watcher")
			return nil
		case <-ticker.C:
			// initNewCache is OS specific.
			// Based on GOOS, will be implemented by either endpoint_linux, or
			// endpoint_windows.
			err := w.initNewCache()
			if err != nil {
				return err
			}

			// Compare the new veths with the old ones.
			created, deleted := w.diffCache()

			// Publish the new veths.
			for _, v := range created {
				w.l.Debug("Endpoint created", zap.Any("veth", v))
				w.p.Publish(common.PubSubEndpoints, NewEndpointEvent(EndpointCreated, v))
			}

			// Publish the deleted veths.
			for _, v := range deleted {
				w.l.Debug("Endpoint deleted", zap.Any("veth", v))
				w.p.Publish(common.PubSubEndpoints, NewEndpointEvent(EndpointDeleted, v))
			}

			// Update the cache and reset the new cache.
			w.current = w.new.deepcopy()
			w.new = nil
		}
	}
}

func (w *Watcher) Stop(_ context.Context) error {
	w.l.Info("Stopping endpoint watcher")
	return nil
}

// Function to differentiate between two caches.
func (w *Watcher) diffCache() (created, deleted []interface{}) {
	// Check if there are any new veths.
	for k, v := range w.new {
		if _, ok := w.current[k]; !ok {
			created = append(created, v)
		}
	}

	// Check if there are any deleted veths.
	for k, v := range w.current {
		if _, ok := w.new[k]; !ok {
			deleted = append(deleted, v)
		}
	}
	return
}
