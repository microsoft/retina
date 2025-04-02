// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache

import (
	"fmt"
	"sync"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type PodInfo struct {
	Name      string
	Namespace string
	Active    bool
}

type StandaloneCache struct {
	rwMutex sync.RWMutex
	l       *log.ZapLogger
	ipToPod map[string]*PodInfo
}

func NewStandaloneCache() *StandaloneCache {
	return &StandaloneCache{
		l:       log.Logger().Named(string("standalone-cache")),
		ipToPod: make(map[string]*PodInfo),
	}
}

func (c *StandaloneCache) GetPod(ip string) *PodInfo {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()

	if pod, exists := c.ipToPod[ip]; exists {
		return pod
	}
	return nil
}

func (c *StandaloneCache) Update(ip string, podInfo *PodInfo) {
	if podInfo != nil {
		c.addPod(ip, podInfo.Name, podInfo.Namespace)
	} else {
		c.deletePod(ip)
	}
}

func (c *StandaloneCache) ResetIPStatuses() {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	for _, podInfo := range c.ipToPod {
		podInfo.Active = false
	}
}

func (c *StandaloneCache) RemoveStaleEntries() {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	for ip, podInfo := range c.ipToPod {
		if !podInfo.Active {
			delete(c.ipToPod, ip)
			c.l.Debug("Removed stale pod IP from cache", zap.String("ip", ip), zap.String("name", podInfo.Name), zap.String("namespace", podInfo.Namespace))
		}
	}
}

func (c *StandaloneCache) addPod(ip, name, namespace string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	existingPod, exists := c.ipToPod[ip]
	newPod := &PodInfo{Name: name, Namespace: namespace, Active: true}

	// Skip adding element if identical
	if exists && existingPod.isEqual(newPod) {
		fmt.Printf("MUDIT")
		existingPod.Active = true
		return
	}

	c.ipToPod[ip] = newPod
	if !exists {
		c.l.Info("Added pod to cache", zap.String("ip", ip), zap.String("name", name), zap.String("namespace", namespace))
	}
}

func (c *StandaloneCache) deletePod(ip string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	if podInfo, exists := c.ipToPod[ip]; exists {
		delete(c.ipToPod, ip)
		c.l.Info("Deleted pod from cache", zap.String("ip", ip), zap.String("name", podInfo.Name), zap.String("namespace", podInfo.Namespace))
	}
}

func (c *PodInfo) isEqual(other *PodInfo) bool {
	return c.Name == other.Name && c.Namespace == other.Namespace
}
