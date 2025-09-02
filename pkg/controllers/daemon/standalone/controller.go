// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"context"
	"fmt"
	"time"

	"github.com/microsoft/retina/pkg/common"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache/standalone"
	"github.com/microsoft/retina/pkg/controllers/daemon/standalone/source"
	"github.com/microsoft/retina/pkg/log"
	sm "github.com/microsoft/retina/pkg/module/metrics/standalone"

	"go.uber.org/zap"
)

type Controller struct {
	// interface for fetching retina endpoint information
	src source.Source
	// cache to hold retina endpoints
	cache *standalone.Cache

	metricsModule *sm.Module
	config        *kcfg.Config
	l             *log.ZapLogger
}

// New creates a new instance of the standalone controller
func New(config *kcfg.Config, cache *standalone.Cache, metricsModule *sm.Module) *Controller {
	var src source.Source

	if config.EnableCrictl {
		src = &source.Ctrinfo{}
	} else {
		src = &source.Statefile{}
	}

	return &Controller{
		src:           src,
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
	runningEps, err := c.src.GetAllEndpoints()
	if err != nil {
		return fmt.Errorf("failed to get running endpoints: %w", err)
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
				return fmt.Errorf("failed to delete retina endpoint for ip=%s: %w", ip, err)
			}
		}
	}

	// Update IPs that are not existing in cache
	for ip, ep := range runningIPs {
		if err := c.cache.UpdateRetinaEndpoint(ep); err != nil {
			return fmt.Errorf("failed to update retina endpoint for ip=%s: %w", ip, err)
		}
	}

	c.metricsModule.Reconcile(ctx)
	c.l.Info("Reconciliation completed")
	return nil
}

// Run starts the controller loop
func (c *Controller) Run(ctx context.Context) {
	c.l.Info("Starting controller")

	ticker := time.NewTicker(c.config.MetricsInterval / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.Stop()
			return
		case <-ticker.C:
			if err := c.Reconcile(ctx); err != nil {
				c.l.Error("failed to reconcile", zap.Error(err))
			}
		}
	}
}

// Stop stops the controller and cleans up resources
func (c *Controller) Stop() {
	c.l.Info("Stopping controller")
	c.cache.Clear()
	c.metricsModule.Clear()
}
