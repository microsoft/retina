// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"context"
	"strings"
	"sync"
	"time"

	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type expireFn func(lbs []string) bool

type updated struct {
	t   time.Time
	lbs []string
}

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=basemetricsobject.go -destination=mock_basemetricsobject.go -package=metrics
type baseMetricInterface interface {
	// This func is used to clean up any resources used by the base metric object
	clean()
	isAdvanced() bool
	sourceCtx() ContextOptionsInterface
	destinationCtx() ContextOptionsInterface
	additionalLabels() []string
	isLocalContext() bool
	// This func is used to track updates to the metric labels. It is called by the child metric object whenever the metric is updated
	updated(lbs []string)
	getLogger() *log.ZapLogger
	// Returns the full set of tracked metric labels, this is expensive so should only be used for testing and debugging purposes
	trackedMetricLabels() [][]string
}

type baseMetricObject struct {
	*sync.RWMutex
	advEnable   bool
	contextMode enrichmentContext
	ctxOptions  *api.MetricsContextOptions
	srcCtx      ContextOptionsInterface
	dstCtx      ContextOptionsInterface
	l           *log.ZapLogger
	lastUpdated map[string]updated
	expireFn    expireFn
	cancelFn    context.CancelFunc
	ctx         context.Context
}

func (b *baseMetricObject) additionalLabels() []string {
	if b.ctxOptions == nil {
		return nil
	}

	return b.ctxOptions.AdditionalLabels
}

func (b *baseMetricObject) trackedMetricLabels() [][]string {
	if b.expireFn == nil {
		return nil
	}

	b.RLock()
	defer b.RUnlock()

	labels := make([][]string, 0, len(b.lastUpdated))
	for _, u := range b.lastUpdated {
		labels = append(labels, u.lbs)
	}

	return labels
}

func (b *baseMetricObject) isAdvanced() bool {
	return b.advEnable
}

func (b *baseMetricObject) sourceCtx() ContextOptionsInterface {
	return b.srcCtx
}

func (b *baseMetricObject) destinationCtx() ContextOptionsInterface {
	return b.dstCtx
}

func (b *baseMetricObject) getLogger() *log.ZapLogger {
	return b.l
}

func (b *baseMetricObject) expire(ttl time.Duration) int {
	if b.expireFn == nil {
		return 0
	}

	b.Lock()
	defer b.Unlock()

	var expired int
	n := make(map[string]updated)

	for k, u := range b.lastUpdated {
		if time.Since(u.t) >= ttl {
			d := b.expireFn(u.lbs)
			if d {
				expired++
			}
		} else {
			n[k] = u
		}
	}

	b.lastUpdated = n

	return expired
}

func (b *baseMetricObject) updated(lbs []string) {
	// no expiration function is defined, so we don't need to track updates
	if b.expireFn == nil {
		return
	}

	k := strings.Join(lbs, "")

	b.Lock()
	defer b.Unlock()

	b.lastUpdated[k] = updated{
		t:   time.Now(),
		lbs: lbs,
	}
}

func newBaseMetricsObject(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext, expire expireFn, ttl time.Duration) *baseMetricObject {
	expireOrInfiniteTTL := expire
	if ttl <= 0 {
		// infinite TTL, so make sure the expiration function is unset
		expireOrInfiniteTTL = nil
	}

	b := baseMetricObject{
		advEnable:   ctxOptions.IsAdvanced(),
		ctxOptions:  ctxOptions,
		l:           fl,
		contextMode: isLocalContext,
		expireFn:    expireOrInfiniteTTL,
	}

	if expireOrInfiniteTTL != nil {
		// only initialize these if we have a valid expiration function to save some memory
		b.RWMutex = &sync.RWMutex{}
		b.lastUpdated = make(map[string]updated)
		ctx, cancel := context.WithCancel(context.Background())
		b.ctx = ctx
		b.cancelFn = cancel
		b.l.Info(
			"Starting metric expiration routine: "+ctxOptions.MetricName,
			zap.Duration("ttl", ttl),
		)
		go func() {
			ticker := time.NewTicker(ttl)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					b.l.Info("Stopping metric expiration routine: " + b.ctxOptions.MetricName)
					return
				case t := <-ticker.C:
					b.l.Debug("Expiring metrics: " + b.ctxOptions.MetricName)
					n := b.expire(ttl)
					b.l.Debug(
						"Metric expiration finished: "+b.ctxOptions.MetricName,
						zap.Time("next_expiration", t.Add(ttl)),
						zap.Int("expired", n),
					)
				}
			}
		}()
	}

	b.populateCtxOptions(ctxOptions)
	return &b
}

func (b *baseMetricObject) populateCtxOptions(ctxOptions *api.MetricsContextOptions) {
	if b.isLocalContext() {
		// when localcontext is enabled, we do not need the context options for both src and dst
		// metrics aggregation will be on a single pod basis and not the src/dst pod combination basis.
		// so we can getaway with just one context type. For this reason we will only use srccontext
		// we can ignore adding destination context.
		if ctxOptions.SourceLabels != nil {
			b.srcCtx = NewCtxOption(ctxOptions.SourceLabels, localCtx)
		}
	} else {
		if ctxOptions.SourceLabels != nil {
			b.srcCtx = NewCtxOption(ctxOptions.SourceLabels, source)
		}

		if ctxOptions.DestinationLabels != nil {
			b.dstCtx = NewCtxOption(ctxOptions.DestinationLabels, destination)
		}
	}
}

func (b *baseMetricObject) isLocalContext() bool {
	return b.contextMode == localContext
}

func (b *baseMetricObject) clean() {
	if b.cancelFn != nil {
		b.cancelFn()
	}
}
