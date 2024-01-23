// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpoint

const (
	endpointCreated string = "endpoint_created"
	endpointDeleted string = "endpoint_deleted"
)

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
