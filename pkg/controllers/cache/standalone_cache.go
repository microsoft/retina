// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache

import (
	"sync"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type PodInfo struct {
	Name      string
	Namespace string
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

func (c *StandaloneCache) ProcessPodInfo(ip string, podInfo *PodInfo) {
	if podInfo != nil {
		c.addPod(ip, podInfo.Name, podInfo.Namespace)
	} else {
		c.deletePod(ip)
	}
}

func (c *StandaloneCache) addPod(ip, name, namespace string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	existingPod, exists := c.ipToPod[ip]
	newPod := &PodInfo{Name: name, Namespace: namespace}

	// Skip adding element if identical
	if exists && *existingPod == *newPod {
		return
	}

	c.ipToPod[ip] = newPod
	c.l.Info("Added pod to cache", zap.String("ip", ip), zap.String("name", name), zap.String("namespace", namespace))
}

func (c *StandaloneCache) deletePod(ip string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	if podInfo, exists := c.ipToPod[ip]; exists {
		delete(c.ipToPod, ip)
		c.l.Info("Deleted pod from cache", zap.String("ip", ip), zap.String("name", podInfo.Name), zap.String("namespace", podInfo.Namespace))
	}
}
