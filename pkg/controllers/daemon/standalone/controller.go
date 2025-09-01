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

type Controller struct {
	// interface for fetching retina endpoint information
	source utils.Source
	// cache to hold retina endpoints
	cache *standalone.Cache

	metricsModule *sm.Module
	config        *kcfg.Config
	l             *log.ZapLogger
}

// New creates a new instance of the standalone controller
func New(config *kcfg.Config, cache *standalone.Cache, metricsModule *sm.Module) *Controller {
	var source utils.Source

	if config.EnableCrictl {
		source = &utils.CtrinfoSource{}
	} else {
		source = &utils.StatefileSource{}
	}

	return &Controller{
		source:        source,
		cache:         cache,
		config:        config,
		metricsModule: metricsModule,
		l:             log.Logger().Named(string("Controller")),
	}
}

// Reconcile syncs the state of the running endpoints with the existing endpoints in cache
func (c *Controller) Reconcile(ctx context.Context) error {
	c.l.Info("Reconciling retina endpoints")

	// Retrieve running pod information from the corresponding source
	runningEps, err := c.source.GetAllEndpoints()
	if err != nil {
		c.l.Error("failed to get endpoints", zap.Error(err))
		return err
	}

	runningIPs := make(map[string]*common.RetinaEndpoint)
	for _, ep := range runningEps {
		ip, err := ep.PrimaryIP()
		if err != nil || ip == "" {
			continue
		}
		runningIPs[ip] = ep
	}

	cachedIPs := c.cache.GetAllIPs()

	// Remove IPs not in the running set
	for _, ip := range cachedIPs {
		if _, exists := runningIPs[ip]; !exists {
			if err := c.cache.DeleteRetinaEndpoint(ip); err != nil {
				c.l.Error("failed to delete retina endpoint", zap.String("ip", ip), zap.Error(err))
				return err
			}
		}
	}

	// Update IPs that are not existing in cache
	for ip, ep := range runningIPs {
		if err := c.cache.UpdateRetinaEndpoint(ep); err != nil {
			c.l.Error("failed to update retina endpoint", zap.String("ip", ip), zap.Error(err))
			return err
		}
	}

	c.metricsModule.Reconcile(ctx)
	c.l.Info("Reconciliation completed")
	return nil
}

// Run starts the controller loop
func (sc *Controller) Run(ctx context.Context) {
	sc.l.Info("Starting controller")

	ticker := time.NewTicker(sc.config.MetricsInterval / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			sc.Stop()
			return
		case <-ticker.C:
			if err := sc.Reconcile(ctx); err != nil {
				sc.l.Error("failed to reconcile", zap.Error(err))
			}
		}
	}
}

// Stop stops the controller and cleans up resources
func (sc *Controller) Stop() {
	sc.l.Info("Stopping controller")
	sc.cache.Clear()
	sc.metricsModule.Clear()
}
