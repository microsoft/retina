// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"bytes"
	"crypto/sha256"
	"fmt"
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

type baseMetricObject struct {
	*sync.Mutex
	advEnable   bool
	contextMode enrichmentContext
	ctxOptions  *api.MetricsContextOptions
	srcCtx      ContextOptionsInterface
	dstCtx      ContextOptionsInterface
	l           *log.ZapLogger
	lastUpdated map[string]updated
	expireFn    expireFn
}

func (b *baseMetricObject) expire(ttl time.Duration) int {
	if b.expireFn == nil {
		return 0
	}

	b.Lock()
	defer b.Unlock()

	var expired int
	n := make(map[string]updated)

	for h, u := range b.lastUpdated {
		if time.Since(u.t) >= ttl {
			d := b.expireFn(u.lbs)
			if d {
				expired++
			}
		} else {
			n[h] = u
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

	var bf bytes.Buffer

	for _, l := range lbs {
		bf.WriteString(l)
	}

	h := sha256.New()
	h.Write(bf.Bytes())

	s := string(h.Sum(nil))

	b.Lock()
	defer b.Unlock()

	b.lastUpdated[s] = updated{
		t:   time.Now(),
		lbs: lbs,
	}
}

func newBaseMetricsObject(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext, expire expireFn, ttl time.Duration) baseMetricObject {
	expireOrInfiniteTTL := expire
	if ttl <= 0 {
		// infinite TTL, so make sure the expiration function is unset
		expireOrInfiniteTTL = nil
	}

	b := baseMetricObject{
		Mutex:       &sync.Mutex{},
		advEnable:   ctxOptions.IsAdvanced(),
		ctxOptions:  ctxOptions,
		l:           fl,
		contextMode: isLocalContext,
		lastUpdated: make(map[string]updated),
		expireFn:    expireOrInfiniteTTL,
	}

	if expireOrInfiniteTTL != nil {
		go func() {
			for {
				b.l.Debug(fmt.Sprintf("Expiring metrics: %s", ctxOptions.MetricName))
				n := b.expire(ttl)
				b.l.Debug(
					fmt.Sprintf("Metric expiration finished: %s", ctxOptions.MetricName),
					zap.Time("next_expiration", time.Now().Add(ttl)),
					zap.Int("expired", n),
				)
				time.Sleep(ttl)
			}
		}()
	}

	b.populateCtxOptions(ctxOptions)
	return b
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
