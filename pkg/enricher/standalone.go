// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	c "github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher/base"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type StandaloneEnricher struct {
	*base.Enricher
}

func newStandalone(ctx context.Context, cache c.CacheInterface) *StandaloneEnricher {
	logger := log.Logger().Named("standalone-enricher")
	return &StandaloneEnricher{
		Enricher: base.NewEnricher(ctx, logger, cache),
	}
}

func NewStandalone(ctx context.Context, cache c.CacheInterface) base.EnricherInterface {
	base.Once.Do(func() {
		base.E = newStandalone(ctx, cache)
		base.Initialized = true
	})
	return base.E
}

func (se *StandaloneEnricher) Enrich(ev *v1.Event) {
	fl := ev.Event.(*flow.Flow)

	if fl.GetIP().GetSource() == "" {
		se.L.Debug("source IP is empty")
		return
	}

	srcObj := se.Cache.GetPodByIP(fl.GetIP().GetSource())
	if srcObj != nil {
		fl.Source = se.GetEndpoint(srcObj)
		se.L.Debug("enriched flow", zap.Any("flow", fl))
	} else {
		fl.Source = nil
	}

	ev.Event = fl
	se.Export(ev)
}

func (se *StandaloneEnricher) Run() {
	se.Enricher.Run(se.Enrich)
}
