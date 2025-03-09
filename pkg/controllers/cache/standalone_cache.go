// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache

import (
	"sync"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

var (
	MaxStandaloneCacheEventSize = 250
)

type Action string

const (
	EventAdd    Action = "Add"
	EventDelete Action = "Delete"
)

type PodInfo struct {
	Name      string
	Namespace string
}

type StandaloneCacheEvent struct {
	Ip      string
	PodInfo PodInfo
	Action  Action
}

type StandaloneCache struct {
	rwMutex      sync.RWMutex
	l            *log.ZapLogger
	ipToPod      map[string]PodInfo
	eventChannel chan StandaloneCacheEvent
}

func NewStandaloneCache() *StandaloneCache {
	return &StandaloneCache{
		l:            log.Logger().Named(string("Standalone Cache")),
		ipToPod:      make(map[string]PodInfo),
		eventChannel: make(chan StandaloneCacheEvent, MaxStandaloneCacheEventSize),
	}
}

func (c *StandaloneCache) AddPod(ip, name, namespace string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	c.ipToPod[ip] = PodInfo{Name: name, Namespace: namespace}
	c.l.Info("Added pod", zap.String("ip", ip), zap.String("name", name), zap.String("namespace", namespace))

	c.eventChannel <- StandaloneCacheEvent{
		Ip: ip,
		PodInfo: PodInfo{
			Name:      name,
			Namespace: namespace,
		},
		Action: EventAdd,
	}
}

func (c *StandaloneCache) GetPod(ip string) *PodInfo {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()

	if pod, exists := c.ipToPod[ip]; exists {
		return &pod
	}
	return nil
}

func (c *StandaloneCache) DeletePod(ip string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	if podInfo, exists := c.ipToPod[ip]; exists {
		delete(c.ipToPod, ip)
		c.l.Info("Deleted pod", zap.String("ip", ip), zap.String("name", podInfo.Name), zap.String("namespace", podInfo.Namespace))

		c.eventChannel <- StandaloneCacheEvent{
			Ip:      ip,
			PodInfo: podInfo,
			Action:  EventDelete,
		}
	}
}

func (c *StandaloneCache) WatchEvents() <-chan StandaloneCacheEvent {
	return c.eventChannel
}
