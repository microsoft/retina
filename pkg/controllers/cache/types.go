// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cache

import (
	"net"

	"github.com/microsoft/retina/pkg/common"
)

//go:generate mockgen -destination=mock_cacheinterface.go -copyright_file=../../lib/ignore_headers.txt -package=cache github.com/microsoft/retina/pkg/controllers/cache CacheInterface
type CacheInterface interface {
	// GetPodByIP returns the retina endpoint for the given IP.
	GetPodByIP(ip string) *common.RetinaEndpoint
	// GetSvcByIP returns the retina service for the given IP.
	GetSvcByIP(ip string) *common.RetinaSvc
	// GetNodeByIP returns the retina node for the given IP.
	GetNodeByIP(ip string) *common.RetinaNode
	// GetObjByIP returns the object for the given IP.
	GetObjByIP(ip string) interface{}
	// GetIPsByNamespace returns the net.IPs for a given namespace.
	GetIPsByNamespace(ns string) []net.IP
	// GetAnnotatedNamespaces returns list of namespaces that are annotated with retina to observe.
	GetAnnotatedNamespaces() []string

	// UpdateRetinaEndpoint updates the retina endpoint in the cache.
	UpdateRetinaEndpoint(ep *common.RetinaEndpoint) error
	// UpdateRetinaSvc updates the retina service in the cache.
	UpdateRetinaSvc(svc *common.RetinaSvc) error
	// UpdateRetinaNode updates the retina node in the cache.
	UpdateRetinaNode(node *common.RetinaNode) error
	AddAnnotatedNamespace(ns string)

	// DeleteRetinaEndpoint deletes the retina endpoint from the cache.
	DeleteRetinaEndpoint(epKey string) error
	// DeleteRetinaSvc deletes the retina service from the cache.
	DeleteRetinaSvc(svcKey string) error
	// DeleteRetinaNode deletes the retina node from the cache.
	DeleteRetinaNode(nodeName string) error
	DeleteAnnotatedNamespace(ns string)
}

type CacheEvent struct {
	// Type is the type of the event.
	Type EventType
	// Obj is the object that the event is about.
	Obj interface{}
}

func NewCacheEvent(t EventType, obj common.PublishObj) *CacheEvent {
	return &CacheEvent{
		Type: t,
		Obj:  obj.DeepCopy(),
	}
}

type EventType int

const (
	// EventTypePodAdded is the event type for a pod add event.
	EventTypePodAdded EventType = iota
	// EventTypePodDeleted is the event type for a pod delete event.
	EventTypePodDeleted
	// EventTypeSvcAdded is the event type for a service add event.
	EventTypeSvcAdded
	// EventTypeSvcDeleted is the event type for a service delete event.
	EventTypeSvcDeleted
	// EventTypeNodeAdded is the event type for a node add event.
	EventTypeNodeAdded
	// EventTypeNodeDeleted is the event type for a node delete event.
	EventTypeNodeDeleted
	// EventTypeAddAPIServerIPs  is the event type for adding API server IPs.
	EventTypeAddAPIServerIPs
	// EventTypeDeleteAPIServerIPs  is the event type for deleting API server IPs.
	EventTypeDeleteAPIServerIPs
)

func (e EventType) String() string {
	switch e {
	case EventTypePodAdded:
		return "pod added"
	case EventTypePodDeleted:
		return "pod deleted"
	case EventTypeSvcAdded:
		return "service added"
	case EventTypeSvcDeleted:
		return "service deleted"
	case EventTypeNodeAdded:
		return "node added"
	case EventTypeNodeDeleted:
		return "node deleted"
	case EventTypeAddAPIServerIPs:
		return "add API server IPs"
	case EventTypeDeleteAPIServerIPs:
		return "delete API server IPs"
	default:
		return "unknown"
	}
}

type objectType int

const (
	TypeEndpoint objectType = iota
	TypeSvc
	TypeNode
)

func (o objectType) String() string {
	switch o {
	case TypeEndpoint:
		return "endpoint"
	case TypeSvc:
		return "service"
	case TypeNode:
		return "node"
	default:
		return "unknown"
	}
}
