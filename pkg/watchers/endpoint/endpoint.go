// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpoint

import (
	"context"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"go.uber.org/zap"
)

type EndpointWatcher struct {
	isRunning bool
	l         *log.ZapLogger
	current   cache
	new       cache
	p         pubsub.PubSubInterface
}

var e *EndpointWatcher

// NewEndpointWatcher creates a new endpoint watcher.
func Watcher() *EndpointWatcher {
	if e == nil {
		e = &EndpointWatcher{
			isRunning: false,
			l:         log.Logger().Named("endpoint-watcher"),
			p:         pubsub.New(),
			current:   make(cache),
		}
	}

	return e
}

func (e *EndpointWatcher) Init(ctx context.Context) error {
	if e.isRunning {
		e.l.Info("endpoint watcher is already running")
		return nil
	}
	e.isRunning = true
	return nil
}

func (e *EndpointWatcher) Stop(ctx context.Context) error {
	if !e.isRunning {
		e.l.Info("endpoint watcher is not running")
		return nil
	}
	e.isRunning = false
	return nil
}

func (e *EndpointWatcher) Refresh(ctx context.Context) error {
	// initNewCache is OS specific.
	// Based on GOOS, will be implemented by either endpoint_linux, or
	// endpoint_windows.
	err := e.initNewCache()
	if err != nil {
		return err
	}

	// Compare the new veths with the old ones.
	created, deleted := e.diffCache()

	// Publish the new veths.
	for _, v := range created {
		e.l.Debug("Endpoint created", zap.Any("veth", v))
		e.p.Publish(common.PubSubEndpoints, NewEndpointEvent(EndpointCreated, v))
	}

	// Publish the deleted veths.
	for _, v := range deleted {
		e.l.Debug("Endpoint deleted", zap.Any("veth", v))
		e.p.Publish(common.PubSubEndpoints, NewEndpointEvent(EndpointDeleted, v))
	}

	// Update the cache and reset the new cache.
	e.current = e.new.deepcopy()
	e.new = nil

	return nil
}

// Function to differentiate between two caches.
func (e *EndpointWatcher) diffCache() (created, deleted []interface{}) {
	// Check if there are any new veths.
	for k, v := range e.new {
		if _, ok := e.current[k]; !ok {
			created = append(created, v)
		}
	}

	// Check if there are any deleted veths.
	for k, v := range e.current {
		if _, ok := e.new[k]; !ok {
			deleted = append(deleted, v)
		}
	}
	return
}
