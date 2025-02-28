// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache

import (
	"sync"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type StandaloneCache struct {
	rwMutex sync.RWMutex
	l       *log.ZapLogger
	ipToPod map[string]PodInfo
}

type PodInfo struct {
	Name      string
	Namespace string
}

func NewStandaloneCache() *StandaloneCache {
	return &StandaloneCache{
		l:       log.Logger().Named(string("Standalone Cache")),
		ipToPod: make(map[string]PodInfo),
	}
}

// AddPod adds pods IP mapping to the cache
func (c *StandaloneCache) AddPod(ip, name, namespace string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	c.ipToPod[ip] = PodInfo{Name: name, Namespace: namespace}
	c.l.Info("Added pod", zap.String("ip", ip), zap.String("name", name), zap.String("namespace", namespace))
}

// GetPod retrieves pod information by IP address
func (c *StandaloneCache) GetPod(ip string) *PodInfo {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()

	if pod, exists := c.ipToPod[ip]; exists {
		return &pod
	}
	return nil
}

// DeletePod removes pods IP mapping from the cache
func (c *StandaloneCache) DeletePod(ip string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	if podInfo, exists := c.ipToPod[ip]; exists {
		delete(c.ipToPod, ip)
		c.l.Info("Deleted pod", zap.String("ip", ip), zap.String("name", podInfo.Name), zap.String("namespace", podInfo.Namespace))
	}
}
