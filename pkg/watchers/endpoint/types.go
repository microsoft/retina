// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpoint

import (
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
)

const (
	watcherName        string = "endpoint-watcher"
	endpointCreated    string = "endpoint_created"
	endpointDeleted    string = "endpoint_deleted"
	defaultRefreshRate        = 30 * time.Second
)

type Watcher struct {
	l           *log.ZapLogger
	current     cache
	new         cache
	p           pubsub.PubSubInterface
	refreshRate time.Duration
}

var w *Watcher

// NewEndpointWatcher creates a new endpoint watcher.
func NewWatcher() *Watcher {
	if w == nil {
		w = &Watcher{
			l:           log.Logger().Named(watcherName),
			p:           pubsub.New(),
			current:     make(cache),
			refreshRate: defaultRefreshRate,
		}
	}

	return w
}

type key struct {
	name         string
	hardwareAddr string
	// Network namespace for linux.
	// Compartment ID for windows.
	netNsID int
}

type cache map[key]interface{}

type EndpointEvent struct {
	// Type is the type of the event.
	Type EventType
	// Obj is the object that the event is about.
	Obj interface{}
}

func NewEndpointEvent(t EventType, obj interface{}) *EndpointEvent {
	return &EndpointEvent{
		Type: t,
		Obj:  obj,
	}
}

type EventType int

const (
	EndpointCreated EventType = iota
	EndpointDeleted
)

func (e EventType) String() string {
	switch e {
	case EndpointCreated:
		return endpointCreated
	case EndpointDeleted:
		return endpointDeleted
	default:
		return "unknown"
	}
}

func (c cache) deepcopy() cache {
	copy := make(cache)
	for k, v := range c {
		copy[k] = v
	}
	return copy
}
