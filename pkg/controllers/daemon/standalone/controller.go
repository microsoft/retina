// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"context"
	"time"

	"github.com/microsoft/retina/pkg/common"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache/standalone"
	"github.com/microsoft/retina/pkg/controllers/daemon/standalone/utils"
	sm "github.com/microsoft/retina/pkg/module/metrics/standalone"

	"github.com/microsoft/retina/pkg/log"

	"go.uber.org/zap"
)

type StandaloneController struct {
	// interface for fetching endpoint information
	source utils.Source
	// cache to hold retina endpoints
	cache *standalone.Cache

	metricsModule *sm.Module
	config        *kcfg.Config
	l             *log.ZapLogger
}

func New(config *kcfg.Config, cache *standalone.Cache, metricsModule *sm.Module) *StandaloneController {
	var source utils.Source

	if config.EnableCrictl {
		source = &utils.CtrinfoSource{}
	} else {
		source = &utils.StatefileSource{}
	}

	return &StandaloneController{
		source:        source,
		cache:         cache,
		config:        config,
		metricsModule: metricsModule,
		l:             log.Logger().Named(string("StandaloneController")),
	}
}

// Reconcile syncs the state of the endpoints with the desired state
func (sc *StandaloneController) Reconcile(ctx context.Context) error {
	sc.l.Info("Starting standalone reconciliation")

	srcEndpoints, err := sc.source.GetAllEndpoints()
	if err != nil {
		sc.l.Error("Failed to get all endpoints", zap.Error(err))
		return err
	}

	srcIPs := make(map[string]*common.RetinaEndpoint, len(srcEndpoints))
	for _, ep := range srcEndpoints {
		ip, err := ep.PrimaryIP()
		if err != nil {
			continue
		}
		if ip == "" {
			continue
		}
		srcIPs[ip] = ep
	}

	cachedIPs := sc.cache.GetAllIPs()

	for _, ip := range cachedIPs {
		if _, exists := srcIPs[ip]; !exists {
			sc.cache.DeleteRetinaEndpoint(ip)
			// sc.metricsModule.RemoveSeries(ip)
		}
	}

	for ip, ep := range srcIPs {
		if err := sc.cache.UpdateRetinaEndpoint(ep); err != nil {
			sc.l.Error("Failed to update retina endpoint", zap.String("ip", ip), zap.Error(err))
			return err
		}
	}
	sc.metricsModule.Reconcile(ctx)

	sc.l.Info("Standalone reconciliation completed")
	return nil
}

func (sc *StandaloneController) Run(ctx context.Context) {
	sc.l.Info("Starting Standalone Controller")

	ticker := time.NewTicker(sc.config.MetricsInterval / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			sc.Stop()
			return
		case <-ticker.C:
			if err := sc.Reconcile(ctx); err != nil {
				sc.l.Error("Failed to reconcile", zap.Error(err))
			}
		}
	}
}

func (sc *StandaloneController) Stop() {
	sc.l.Info("Stopping Standalone Controller")
	sc.cache.Clear()
	sc.metricsModule.Clear()
}
