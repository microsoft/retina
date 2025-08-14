// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	ctr "github.com/microsoft/retina/pkg/enricher/ctrinfo"
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

type Action string

const (
	AddEvent    Action = "add"
	DeleteEvent Action = "delete"
)

type StandaloneEvent struct {
	IP     string
	Action Action
}

type StandaloneEnricher struct {
	cfg          *config.Config
	ctx          context.Context
	l            *log.ZapLogger
	cache        *cache.StandaloneCache
	eventChannel chan StandaloneEvent
}

func NewEnricher(ctx context.Context, standaloneCache *cache.StandaloneCache, cfg *config.Config) *StandaloneEnricher {
	return &StandaloneEnricher{
		cfg:          cfg,
		ctx:          ctx,
		l:            log.Logger().Named("standalone-enricher"),
		cache:        standaloneCache,
		eventChannel: make(chan StandaloneEvent, MaxStandaloneCacheEventSize),
	}
}

func NewStandaloneEnricher(ctx context.Context, standaloneCache *cache.StandaloneCache, cfg *config.Config) *StandaloneEnricher {
	localOnce.Do(func() {
		se = NewEnricher(ctx, standaloneCache, cfg)
	})
	return se
}

func StandaloneInstance() *StandaloneEnricher {
	return se
}

func (e *StandaloneEnricher) Run() {
	e.l.Info("Running standalone enricher")
	if e.cfg.EnableCrictl {
		e.l.Info("Using crictl enrichment")
	} else {
		e.l.Info("Using statefile enrichment")
	}

	go func() {
		for {
			select {
			case <-e.ctx.Done():
				e.l.Info("Standalone enricher shutting down...")
				return
			case event, ok := <-e.eventChannel:
				if !ok {
					e.l.Info("Event channel closed, stopping event processing")
					return
				}
				switch event.Action {
				case AddEvent:
					e.l.Debug("Processing add event", zap.String("ip", event.IP))
					e.enrich(event.IP)
				case DeleteEvent:
					e.l.Debug("Processing delete event", zap.String("ip", event.IP))
					e.cache.Update(event.IP, nil)
				default:
					e.l.Warn("Unknown event action", zap.String("action", string(event.Action)))
				}
			}
		}
	}()
}

func (e *StandaloneEnricher) enrich(ip string) {
	var podInfo *cache.PodInfo
	var err error

	if e.cfg.EnableCrictl {
		podInfo, err = ctr.GetPodInfo(ip)
	} else {
		podInfo, err = sf.GetPodInfo(ip, sf.StateFileLocation)
	}

	if err != nil {
		e.l.Error("Failed to get pod info", zap.String("ip", ip), zap.Error(err))
		return
	}
	e.cache.Update(ip, podInfo)
}

func (e *StandaloneEnricher) GetPodInfo(ip string) *cache.PodInfo {
	return e.cache.GetPod(ip)
}

func (e *StandaloneEnricher) PublishEvent(ip string, action Action) error {
	select {
	case e.eventChannel <- StandaloneEvent{IP: ip, Action: action}:
		return nil
	default:
		e.l.Warn("Event channel full, dropping event", zap.String("ip", ip))
		return ErrEventChannelFull
	}
}

func (e *StandaloneEnricher) RemoveStaleEntries() {
	e.cache.ForEach(func(ip string, podInfo *cache.PodInfo) {
		if time.Since(podInfo.LastUpdate) > e.cache.TTL() {
			e.l.Info("Removing stale entry from cache", zap.String("ip", ip))
			err := e.PublishEvent(ip, DeleteEvent)
			if err != nil {
				e.l.Error("Failed to publish delete event", zap.String("ip", ip), zap.Error(err))
			}
		}
	})
}

func (e *StandaloneEnricher) Stop() {
	e.l.Info("Stopping standalone enricher...")
	close(e.eventChannel)
}
