// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"errors"
	"sync"

	"github.com/microsoft/retina/pkg/controllers/cache"
	sf "github.com/microsoft/retina/pkg/enricher/statefile"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

var (
	se        *StandaloneEnricher
	localOnce sync.Once
)

var (
	MaxStandaloneCacheEventSize = 250
	ErrEventChannelFull         = errors.New("event channel is full, event dropped")
)

type StandaloneEvent struct {
	Ip string
}

type StandaloneEnricher struct {
	GenericEnricher
	cache        *cache.StandaloneCache
	eventChannel chan StandaloneEvent
	wg           sync.WaitGroup
}

func NewStandaloneEnricher(ctx context.Context, cache *cache.StandaloneCache) *StandaloneEnricher {
	localOnce.Do(func() {
		se = &StandaloneEnricher{
			GenericEnricher: GenericEnricher{
				ctx: ctx,
				l:   log.Logger().Named("standalone-enricher"),
			},
			cache:        cache,
			eventChannel: make(chan StandaloneEvent, MaxStandaloneCacheEventSize),
		}
	})
	return se
}

func StandaloneInstance() *StandaloneEnricher {
	return se
}

func (e *StandaloneEnricher) Run(ctx context.Context) {
	e.l.Info("Running standalone enricher")
	e.wg.Add(1)

	go func() {
		defer e.wg.Done()
		for {
			select {
			case <-ctx.Done():
				e.l.Info("Standalone enricher shutting down...")
				return
			case event := <-e.eventChannel:
				e.processEvent(event.Ip)
			default:
				e.l.Error("Unknown standalone cache event")
			}
		}
	}()
}

func (c *StandaloneEnricher) processEvent(ip string) {
	podInfo, err := sf.GetPodInfo(ip, sf.State_file_location)
	if err != nil {
		c.l.Error("Failed to get pod info", zap.String("ip", ip), zap.Error(err))
		return
	}
	c.cache.ProcessPodInfo(ip, podInfo)
}

func (c *StandaloneEnricher) GetPodInfo(ip string) *cache.PodInfo {
	return c.cache.GetPod(ip)
}

func (c *StandaloneEnricher) PublishEvent(ip string) error {
	select {
	case c.eventChannel <- StandaloneEvent{Ip: ip}:
		return nil
	default:
		c.l.Warn("Event channel full, dropping event", zap.String("ip", ip))
		return ErrEventChannelFull
	}
}

func (e *StandaloneEnricher) Stop() {
	e.l.Info("Stopping standalone enricher...")
	e.wg.Wait()
	e.l.Info("Standalone enricher stopped")
}
