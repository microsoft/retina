// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
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

type StandaloneEnricher struct {
	GenericEnricher
	cache *cache.StandaloneCache
	wg    sync.WaitGroup
}

func NewStandaloneEnricher(ctx context.Context, cache *cache.StandaloneCache) *StandaloneEnricher {
	localOnce.Do(func() {
		se = &StandaloneEnricher{
			GenericEnricher: GenericEnricher{
				ctx: ctx,
				l:   log.Logger().Named("standalone-enricher"),
			},
			cache: cache,
		}
	})
	return se
}

func StandaloneInstance() *StandaloneEnricher {
	return se
}

func (e *StandaloneEnricher) Run(ctx context.Context) {
	e.l.Info("Running standalone enricher")

	eventsCh := e.cache.WatchEvents()

	go func() {
		for {
			select {
			case <-ctx.Done():
				e.l.Info("Standalone enricher shutting down...")
				return
			case event := <-eventsCh:
				switch event.Action {
				case cache.EventAdd:
					e.l.Debug("Enriching pod", zap.String("ip", event.Ip))
					e.ProcessEvent(event.Ip, event.Action)
				case cache.EventDelete:
					e.l.Debug("Deleting pod", zap.String("ip", event.Ip))
					e.ProcessEvent(event.Ip, event.Action)
				default:
					e.l.Error("Unknown standalone cache event", zap.String("action", string(event.Action)))
				}
			}
		}
	}()
}

func (e *StandaloneEnricher) ProcessEvent(ip string, action cache.Action) {
	if action == cache.EventAdd {
		podInfo, err := sf.GetPodInfo(ip, sf.State_file_location)
		if err != nil {
			e.l.Error("Failed to get pod info", zap.String("ip", ip), zap.Error(err))
			return
		}
		e.l.Debug("Adding pod to cache", zap.String("ip", ip))
		e.cache.AddPod(ip, podInfo.Name, podInfo.Namespace)
	} else if action == cache.EventDelete {
		e.l.Debug("Deleting pod from cache", zap.String("ip", ip))
		e.cache.DeletePod(ip)
	}
	e.cache.PublishEvent(ip, action)
}

func (e *StandaloneEnricher) Stop() {
	e.l.Info("Stopping standalone enricher...")
	e.wg.Wait()
	e.l.Info("Standalone enricher stopped")
}
