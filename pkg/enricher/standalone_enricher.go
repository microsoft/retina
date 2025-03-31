// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"errors"
	"sync"

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

type StandaloneEvent struct {
	IP string
}

type StandaloneEnricher struct {
	cfg          *config.Config
	ctx          context.Context
	l            *log.ZapLogger
	cache        *cache.StandaloneCache
	eventChannel chan StandaloneEvent
}

func NewStandaloneEnricher(ctx context.Context, cache *cache.StandaloneCache, cfg *config.Config) *StandaloneEnricher {
	localOnce.Do(func() {
		se = &StandaloneEnricher{
			cfg:          cfg,
			ctx:          ctx,
			l:            log.Logger().Named("standalone-enricher"),
			cache:        cache,
			eventChannel: make(chan StandaloneEvent, MaxStandaloneCacheEventSize),
		}
	})
	return se
}

func StandaloneInstance() *StandaloneEnricher {
	return se
}

func (e *StandaloneEnricher) Run() {
	e.l.Info("Running standalone enricher")

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
				e.l.Debug("Processing event", zap.String("ip", event.IP))
				e.processEvent(event.IP)
			}
		}
	}()
}

func (e *StandaloneEnricher) processEvent(ip string) {
	var podInfo *cache.PodInfo
	var err error

	if e.cfg.EnableCriCtl {
		e.l.Info("Using containerd to discover pod info")
		podInfo, err = ctr.GetPodInfo(ip)
	} else {
		podInfo, err = sf.GetPodInfo(ip, sf.StateFileLocation)
	}

	if err != nil {
		e.l.Error("Failed to get pod info", zap.String("ip", ip), zap.Error(err))
		return
	}
	e.cache.ProcessPodInfo(ip, podInfo)
}

func (e *StandaloneEnricher) GetPodInfo(ip string) *cache.PodInfo {
	return e.cache.GetPod(ip)
}

func (e *StandaloneEnricher) PublishEvent(ip string) error {
	select {
	case e.eventChannel <- StandaloneEvent{IP: ip}:
		return nil
	default:
		e.l.Warn("Event channel full, dropping event", zap.String("ip", ip))
		return ErrEventChannelFull
	}
}

func (e *StandaloneEnricher) Stop() {
	e.l.Info("Stopping standalone enricher...")
	close(e.eventChannel)
}
