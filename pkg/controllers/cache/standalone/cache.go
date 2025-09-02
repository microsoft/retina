// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"fmt"
	"net"
	"sync"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type Cache struct {
	mu sync.RWMutex
	l  *log.ZapLogger
	// ipToEndpoint is a map of IP addresses to RetinaEndpoints (namespace/name)
	ipToEndpoint map[string]*common.RetinaEndpoint
}

// New returns a new instance of Cache
func New() *Cache {
	c := &Cache{
		l:            log.Logger().Named("Cache"),
		ipToEndpoint: make(map[string]*common.RetinaEndpoint),
	}
	return c
}

// GetAllIPs returns a list of all IPs in the cache
func (c *Cache) GetAllIPs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ips := make([]string, 0, len(c.ipToEndpoint))
	for ip := range c.ipToEndpoint {
		ips = append(ips, ip)
	}
	return ips
}

// GetPodByIP returns the retina endpoint for the given IP
func (c *Cache) GetPodByIP(ip string) *common.RetinaEndpoint {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ipToEndpoint[ip]
}

// UpdateRetinaEndpoint updates the cache with the given retina endpoint
func (c *Cache) UpdateRetinaEndpoint(ep *common.RetinaEndpoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.updateEndpoint(ep)
}

// updateEndpoint updates the cache if there is a new retina endpoint
func (c *Cache) updateEndpoint(ep *common.RetinaEndpoint) error {
	ip, err := ep.PrimaryIP()
	if err != nil {
		c.l.Error("error getting IP for retina endpoint", zap.Error(err))
		return fmt.Errorf("failed to get IP from retina endpoint %s: %w", ep.Key(), err)
	}

	if pod, exists := c.ipToEndpoint[ip]; exists {
		if pod.Name() == ep.Name() && pod.Namespace() == ep.Namespace() {
			return nil
		}
	}
	c.ipToEndpoint[ip] = ep
	c.l.Info("Added retina endpoint to cache", zap.String("ip", ip), zap.String("namespace", ep.Namespace()), zap.String("name", ep.Name()))
	return nil
}

// DeleteRetinaEndpoint deletes the given retina endpoint from the cache
func (c *Cache) DeleteRetinaEndpoint(epKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteEndpoint(epKey)
	return nil
}

// deleteEndpoint deletes the given retina endpoint from the cache
func (c *Cache) deleteEndpoint(epKey string) {
	if ep, exists := c.ipToEndpoint[epKey]; exists {
		delete(c.ipToEndpoint, epKey)
		c.l.Info("Deleted retina endpoint from cache", zap.String("ip", epKey), zap.String("namespace", ep.Namespace()), zap.String("name", ep.Name()))
	}
}

// Clear resets the ip to endpoint map
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ipToEndpoint = make(map[string]*common.RetinaEndpoint)
	c.l.Info("Cleared all retina endpoints from cache")
}

// No op
func (c *Cache) GetSvcByIP(_ string) *common.RetinaSvc   { return nil }
func (c *Cache) GetNodeByIP(_ string) *common.RetinaNode { return nil }
func (c *Cache) GetObjByIP(_ string) interface{}         { return nil }
func (c *Cache) GetIPsByNamespace(_ string) []net.IP     { return nil }
func (c *Cache) GetAnnotatedNamespaces() []string        { return nil }

func (c *Cache) UpdateRetinaSvc(_ *common.RetinaSvc) error   { return nil }
func (c *Cache) DeleteRetinaSvc(_ string) error              { return nil }
func (c *Cache) UpdateRetinaNode(_ *common.RetinaNode) error { return nil }
func (c *Cache) DeleteRetinaNode(_ string) error             { return nil }
func (c *Cache) AddAnnotatedNamespace(_ string)              {}
func (c *Cache) DeleteAnnotatedNamespace(_ string)           {}
