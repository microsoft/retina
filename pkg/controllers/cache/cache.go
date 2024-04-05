// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cache

import (
	"fmt"
	"net"
	"sort"
	"sync"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"go.uber.org/zap"
)

type Cache struct {
	sync.RWMutex
	l *log.ZapLogger
	// endpointMap is a map of pod key (namespace/name) to RetinaEndpoint
	epMap map[string]*common.RetinaEndpoint

	// svcMap is a map of service key (namespace/name) to RetinaSvc
	svcMap map[string]*common.RetinaSvc

	// ipToEpKey is a map of pod IP to list of pod keys (namespace/name)
	ipToEpKey map[string]string

	// ipToSvcKey is a map of service IP to service key (namespace/name)
	ipToSvcKey map[string]string

	// nodeMap is a map of node name to Node
	nodeMap map[string]*common.RetinaNode

	// ipToNodeName is a map of node IP to node name
	ipToNodeName map[string]string

	// nsMap is a map of namespace to no of pods in the namespace
	nsMap map[string]int

	// nsAnnotated is a map of annotated namespaces to watch
	nsAnnotated map[string]bool

	pubsub pubsub.PubSubInterface
}

// NewCache returns a new instance of Cache.
func New(p pubsub.PubSubInterface) *Cache {
	c := &Cache{
		l:            log.Logger().Named(string("Cache")),
		epMap:        make(map[string]*common.RetinaEndpoint),
		svcMap:       make(map[string]*common.RetinaSvc),
		ipToEpKey:    make(map[string]string),
		ipToSvcKey:   make(map[string]string),
		nodeMap:      make(map[string]*common.RetinaNode),
		ipToNodeName: make(map[string]string),
		nsMap:        make(map[string]int),
		nsAnnotated:  make(map[string]bool),
		pubsub:       p,
	}

	cbFunc := pubsub.CallBackFunc(c.SubscribeAPIServerFn)
	c.pubsub.Subscribe(common.PubSubAPIServer, &cbFunc)
	return c
}

// GetPodByIP returns the retina endpoint for the given IP.
func (c *Cache) GetPodByIP(ip string) *common.RetinaEndpoint {
	c.RLock()
	defer c.RUnlock()

	obj := c.getObjByIPType(ip, TypeEndpoint)
	switch obj := obj.(type) {
	case *common.RetinaEndpoint:
		return obj
	default:
		return nil
	}
}

// GetSvcByIP returns the retina service for the given IP.
func (c *Cache) GetSvcByIP(ip string) *common.RetinaSvc {
	c.RLock()
	defer c.RUnlock()

	obj := c.getObjByIPType(ip, TypeSvc)
	switch obj := obj.(type) {
	case *common.RetinaSvc:
		return obj
	default:
		return nil
	}
}

// GetNodeByIP returns the retina node for the given IP.
func (c *Cache) GetNodeByIP(ip string) *common.RetinaNode {
	c.RLock()
	defer c.RUnlock()

	obj := c.getObjByIPType(ip, TypeNode)
	switch obj := obj.(type) {
	case *common.RetinaNode:
		return obj
	default:
		return nil
	}
}

// getObjByIPType returns the retina endpoint for the given IP.
func (c *Cache) getObjByIPType(ip string, t objectType) interface{} {
	switch t {
	case TypeEndpoint:
		podKey, ok := c.ipToEpKey[ip]
		if !ok {
			c.l.Debug("pod not found for IP", zap.String("ip", ip))
			return nil
		}

		ep, ok := c.epMap[podKey]
		if ok {
			c.l.Debug("pod found for IP", zap.String("ip", ip), zap.String("pod", podKey))
			return ep
		}
	case TypeSvc:
		svcKey, ok := c.ipToSvcKey[ip]
		if !ok {
			c.l.Debug("service not found for IP", zap.String("ip", ip))
			return nil
		}

		svc, ok := c.svcMap[svcKey]
		if ok {
			c.l.Debug("service found for IP", zap.String("ip", ip), zap.String("svc", svcKey))
			return svc
		}
	case TypeNode:
		nodeName, ok := c.ipToNodeName[ip]
		if !ok {
			c.l.Debug("node not found for IP", zap.String("ip", ip))
			return nil
		}

		node, ok := c.nodeMap[nodeName]
		if ok {
			c.l.Debug("node found for IP", zap.String("ip", ip), zap.String("node", nodeName))
			return node
		}
	}

	return nil
}

// GetObjByIP returns the retina object for the given IP.
func (c *Cache) GetObjByIP(ip string) interface{} {
	if ep := c.GetPodByIP(ip); ep != nil {
		c.l.Debug("pod found for IP", zap.String("ip", ip), zap.String("pod Name", ep.Key()))
		return ep
	}

	if svc := c.GetSvcByIP(ip); svc != nil {
		return svc
	}

	if node := c.GetNodeByIP(ip); node != nil {
		return node
	}

	return nil
}

func (c *Cache) GetIPsByNamespace(ns string) []net.IP {
	c.RLock()
	defer c.RUnlock()

	var ips []net.IP
	unique := make(map[string]struct{})
	for _, ep := range c.epMap {
		if ep.Namespace() == ns {
			ip, err := ep.PrimaryNetIP()
			if err != nil {
				c.l.Error("GetIPsByNamespace: error getting primary IP for pod", zap.String("pod", ep.Key()), zap.Error(err))
				continue
			}

			if _, ok := unique[ip.String()]; !ok {
				ips = append(ips, ip)
				unique[ip.String()] = struct{}{}
			}
		}
	}

	return ips
}

// UpdateRetinaEndpoint updates the cache with the given retina endpoint.
func (c *Cache) UpdateRetinaEndpoint(ep *common.RetinaEndpoint) error {
	c.Lock()
	defer c.Unlock()

	return c.updateEndpoint(ep)
}

// updateEndpoint updates the cache with the given retina endpoint.
func (c *Cache) updateEndpoint(ep *common.RetinaEndpoint) error {
	ip, err := ep.PrimaryIP()
	if err != nil {
		c.l.Error("updateEndpoint: error getting primary IP for pod", zap.String("pod", ep.Key()), zap.Error(err))
		return err
	}
	// delete if any existing object is using this IP
	// send a delete event for the existing object
	err = c.deleteByIP(ip, ep.Key())
	if err != nil {
		c.l.Error("updateEndpoint: error deleting existing object for IP",
			zap.String("pod", ep.Key()),
			zap.String("ip", ip),
			zap.Error(err),
		)
		return err
	}

	c.epMap[ep.Key()] = ep
	c.ipToEpKey[ip] = ep.Key()

	// notify pubsub that the endpoint has been updated
	c.publish(EventTypePodAdded, ep)

	return nil
}

// UpdateRetinaSvc updates the cache with the given retina service.
func (c *Cache) UpdateRetinaSvc(svc *common.RetinaSvc) error {
	c.Lock()
	defer c.Unlock()

	return c.updateSvc(svc)
}

// updateSvc updates the cache with the given retina service.
func (c *Cache) updateSvc(svc *common.RetinaSvc) error {
	// TODO check if a service already exists here
	ip, err := svc.GetPrimaryIP()
	if err != nil {
		c.l.Error("updateSvc: error getting primary IP for service", zap.String("service", svc.Key()), zap.Error(err))
		return err
	}

	// delete if any existing object is using this IP
	// send a delete event for the existing object
	err = c.deleteByIP(ip, svc.Key())
	if err != nil {
		c.l.Error("updateSvc: error deleting existing object for IP",
			zap.String("svc", svc.Key()),
			zap.String("ip", ip),
			zap.Error(err),
		)
		return err
	}

	c.ipToSvcKey[ip] = svc.Key()
	c.svcMap[svc.Key()] = svc

	// notify pubsub
	c.publish(EventTypeSvcAdded, svc)

	return nil
}

// UpdateRetinaNode updates the cache with the given retina node.
func (c *Cache) UpdateRetinaNode(node *common.RetinaNode) error {
	c.Lock()
	defer c.Unlock()

	return c.updateNode(node)
}

// updateNode updates the cache with the given retina node.
func (c *Cache) updateNode(node *common.RetinaNode) error {
	ip := node.IPString()

	// delete if any existing object is using this IP
	// send a delete event for the existing object
	err := c.deleteByIP(ip, node.Name())
	if err != nil {
		c.l.Error("updateNode: error deleting existing object for IP",
			zap.String("node", node.Name()),
			zap.String("ip", ip),
			zap.Error(err),
		)
		return err
	}

	c.nodeMap[node.Name()] = node
	c.ipToNodeName[node.IPString()] = node.Name()

	// notify pubsub
	c.publish(EventTypeNodeAdded, node)

	return nil
}

// DeleteRetinaEndpoint deletes the given retina endpoint from the cache.
func (c *Cache) DeleteRetinaEndpoint(epKey string) error {
	c.Lock()
	defer c.Unlock()

	return c.deleteEndpoint(epKey)
}

// deleteEndpoint deletes the given retina endpoint from the cache.
func (c *Cache) deleteEndpoint(epKey string) error {
	ep, ok := c.epMap[epKey]
	if !ok {
		c.l.Debug("endpoint not found in cache", zap.String("endpoint", epKey))
		// ignore the error if the endpoint is not found
		return nil
	}

	ip, err := ep.PrimaryIP()
	if err != nil {
		c.l.Error("error getting primary IP for pod", zap.String("pod", ep.Key()), zap.Error(err))
		return err
	}

	delete(c.epMap, epKey)
	delete(c.ipToEpKey, ip)

	c.publish(EventTypePodDeleted, ep)

	return nil
}

// DeleteRetinaSvc deletes the given retina service from the cache.
func (c *Cache) DeleteRetinaSvc(svcKey string) error {
	c.Lock()
	defer c.Unlock()

	return c.deleteSvc(svcKey)
}

// deleteScv deletes the given retina service from the cache.
func (c *Cache) deleteSvc(svcKey string) error {
	svc, ok := c.svcMap[svcKey]
	if !ok {
		c.l.Debug("service not found in cache", zap.String("service", svcKey))
		return fmt.Errorf("service not found in cache: %s", svcKey)
	}

	ip, err := svc.GetPrimaryIP()
	if err != nil {
		c.l.Error("error getting primary IP for service", zap.String("service", svc.Key()), zap.Error(err))
		return err
	}

	delete(c.svcMap, svcKey)
	delete(c.ipToSvcKey, ip)

	// notify pubsub
	c.publish(EventTypeSvcDeleted, svc)

	return nil
}

// DeleteRetinaNode deletes the given retina node from the cache.
func (c *Cache) DeleteRetinaNode(nodeName string) error {
	c.Lock()
	defer c.Unlock()

	return c.deleteNode(nodeName)
}

// deleteNode deletes the given retina node from the cache.
func (c *Cache) deleteNode(nodeName string) error {
	node, ok := c.nodeMap[nodeName]
	if !ok {
		c.l.Debug("node not found in cache", zap.String("node", nodeName))
		return fmt.Errorf("node not found in cache: %s", nodeName)
	}

	delete(c.nodeMap, nodeName)
	delete(c.ipToNodeName, node.IPString())

	c.publish(EventTypeNodeDeleted, node)

	return nil
}

// deleteByIP deletes the given IP from the cache.
func (c *Cache) deleteByIP(ip, key string) error {
	if svcKey, ok := c.ipToSvcKey[ip]; ok {
		if svcKey == key {
			return nil
		}
		c.l.Debug("deleting service by IP", zap.String("ip", ip), zap.String("service", svcKey))
		return c.deleteSvc(svcKey)
	}
	if epKey, ok := c.ipToEpKey[ip]; ok {
		if epKey == key {
			return nil
		}
		c.l.Debug("deleting pod by IP", zap.String("ip", ip), zap.String("pod", epKey))
		return c.deleteEndpoint(epKey)
	}
	if nodeName, ok := c.ipToNodeName[ip]; ok {
		if nodeName == key {
			return nil
		}
		c.l.Debug("deleting node by IP", zap.String("ip", ip), zap.String("node", nodeName))
		return c.deleteNode(nodeName)
	}

	c.l.Debug("IP not found in cache", zap.String("ip", ip))
	return nil
}

func (c *Cache) publish(t EventType, obj common.PublishObj) {
	if c.pubsub == nil {
		c.l.Warn("pubsub not initialized, skipping publish", zap.String("event", t.String()))
		return
	}
	cev := NewCacheEvent(t, obj)
	topic := common.PubSubPods
	if t == EventTypeSvcAdded || t == EventTypeSvcDeleted {
		topic = common.PubSubSvc
	} else if t == EventTypeNodeAdded || t == EventTypeNodeDeleted {
		topic = common.PubSubNode
	}

	go func() {
		c.pubsub.Publish(topic, cev)
	}()
}

func (c *Cache) SubscribeAPIServerFn(obj interface{}) {
	event := obj.(*CacheEvent)
	if event == nil {
		return
	}

	apiServer := event.Obj.(*common.APIServerObject)
	if apiServer == nil || apiServer.EP == nil {
		c.l.Debug("invalid or nil APIServer object in callback function")
		return
	}

	c.Lock()
	defer c.Unlock()

	switch event.Type {
	case EventTypeAddAPIServerIPs:
		err := c.updateEndpoint(apiServer.EP)
		if err != nil {
			c.l.Error("error updating APIServer endpoint in callback function", zap.Error(err))
		}
	case EventTypeDeleteAPIServerIPs:
		err := c.deleteEndpoint(apiServer.EP.Key())
		if err != nil {
			c.l.Error("error deleting APIServer endpoint in callback function", zap.Error(err))
		}
	default:
		c.l.Warn("invalid event type in callback function", zap.String("event", event.Type.String()))

	}
}

func (c *Cache) DeleteAnnotatedNamespace(ns string) {
	c.Lock()
	defer c.Unlock()

	delete(c.nsAnnotated, ns)
}

func (c *Cache) AddAnnotatedNamespace(ns string) {
	c.Lock()
	defer c.Unlock()

	c.nsAnnotated[ns] = true
}

func (c *Cache) GetAnnotatedNamespaces() []string {
	c.RLock()
	defer c.RUnlock()
	ns := make([]string, 0, len(c.nsAnnotated))
	for k := range c.nsAnnotated {
		ns = append(ns, k)
	}
	sort.Strings(ns)
	return ns
}
