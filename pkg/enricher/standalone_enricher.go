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

type StandaloneEnricher struct {
	GenericEnricher
	cache *cache.StandaloneCache
	wg    sync.WaitGroup
}

func NewStandaloneEnricher(ctx context.Context, cache *cache.StandaloneCache) *StandaloneEnricher {
	return &StandaloneEnricher{
		GenericEnricher: GenericEnricher{
			ctx: ctx,
			l:   log.Logger().Named("standalone-enricher"),
		},
		cache: cache,
	}
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
					e.l.Debug("Enriching pod", zap.String("ip", event.Ip), zap.Any("podInfo", event.PodInfo))
					e.enrichCache(event.Ip)
				case cache.EventDelete:
					e.l.Debug("Deleting pod", zap.String("ip", event.Ip), zap.Any("podInfo", event.PodInfo))
					e.DeletePod(event.Ip)
				default:
					e.l.Error("Unknown standalone cache event", zap.String("action", string(event.Action)))
				}
			}
		}
	}()
}

func (e *StandaloneEnricher) enrichCache(ip string) {
	podInfo, err := sf.GetPodInfo(ip, sf.State_file_location)
	if err != nil {
		e.l.Error("Failed to get pod info", zap.String("ip", ip), zap.Error(err))
	}

	e.l.Debug("Adding pod to cache", zap.String("ip", ip), zap.Any("podInfo", podInfo))
	e.cache.AddPod(ip, podInfo.Name, podInfo.Namespace)
}

func (e *StandaloneEnricher) DeletePod(ip string) {
	e.cache.DeletePod(ip)
}

func (e *StandaloneEnricher) Stop() {
	e.l.Info("Stopping standalone enricher...")
	e.wg.Wait()
	e.l.Info("Standalone enricher stopped")
}
