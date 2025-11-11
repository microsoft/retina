// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"

	fl "github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	c "github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher/base"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type StandardEnricher struct {
	*base.Enricher
}

func newStandard(ctx context.Context, cache c.CacheInterface) *StandardEnricher {
	logger := log.Logger().Named("standard-enricher")
	return &StandardEnricher{
		Enricher: base.NewEnricher(ctx, logger, cache),
	}
}

func NewStandard(ctx context.Context, cache c.CacheInterface) base.EnricherInterface {
	base.Once.Do(func() {
		base.E = newStandard(ctx, cache)
		base.Initialized = true
	})
	return base.E
}

// Enrich takes the flow and enriches it with the information from the cache
func (se *StandardEnricher) Enrich(ev *v1.Event) {
	flow := ev.Event.(*fl.Flow)

	// IPversion is a enum in the flow proto
	// 0: IPVersion_IP_NOT_USED
	// 1: IPVersion_IPv4
	// 2: IPVersion_IPv6
	if flow.GetIP().GetIpVersion() > 1 {
		se.L.Error("IP version is not supported", zap.Any("IPVersion", flow.GetIP().GetIpVersion()))
		return
	}
	if flow.GetIP().GetSource() == "" {
		se.L.Debug("source IP is empty")
		return
	}
	srcObj := se.Cache.GetObjByIP(flow.GetIP().GetSource())
	if srcObj != nil {
		flow.Source = se.GetEndpoint(srcObj)
	}

	if flow.GetIP().GetDestination() == "" {
		se.L.Debug("destination IP is empty")
		return
	}

	dstObj := se.Cache.GetObjByIP(flow.GetIP().GetDestination())
	if dstObj != nil {
		flow.Destination = se.GetEndpoint(dstObj)
	}

	ev.Event = flow
	se.L.Debug("enriched flow", zap.Any("flow", flow))
	se.Export(ev)
}

func (se *StandardEnricher) Run() {
	se.Enricher.Run(se.Enrich)
}
