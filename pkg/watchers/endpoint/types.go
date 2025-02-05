// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpoint

import (
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
)

const (
	watcherName            = "endpoint-watcher"
	endpointCreated string = "endpoint_created"
	endpointDeleted string = "endpoint_deleted"
)

type Watcher struct {
	l *log.ZapLogger
	p pubsub.PubSubInterface
}

// NewWatcher creates a new endpoint watcher.
func NewWatcher() *Watcher {
	w := &Watcher{
		l: log.Logger().Named(watcherName),
		p: pubsub.New(),
	}

	return w
}

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
