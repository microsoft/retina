// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package standalone

import (
	"context"
	"strings"
	"sync"

	"github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	mm "github.com/microsoft/retina/pkg/module/metrics"

	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/module/metrics"

	"go.uber.org/zap"
)

const (
	HNS string = "hns"
)

var (
	m               *Module
	once            sync.Once
	requiredMetrics = mm.DefaultStandaloneMetrics()
)

type Module struct {
	*sync.RWMutex
	// ctx is the parent context
	ctx context.Context

	// l is the zaplogger
	l *log.ZapLogger

	// enricher to read events from
	enricher enricher.EnricherInterface

	// registry is the advanced metrics registry
	registry map[string]metrics.AdvMetricsInterface
}

func InitModule(ctx context.Context, enricher enricher.EnricherInterface) *Module {
	once.Do(func() {
		m = &Module{
			RWMutex:  &sync.RWMutex{},
			l:        log.Logger().Named("StandaloneMetricModule"),
			enricher: enricher,
			registry: make(map[string]metrics.AdvMetricsInterface),
		}
		m.updateMetricsContext()
	})

	return m
}

func (m *Module) Reconcile(ctx context.Context) {
	go func() {
		m.Lock()
		m.ctx = ctx
		m.Unlock()

		m.l.Info("Reconciling metric module")

		evReader := m.enricher.ExportReader()
		for {
			ev := evReader.NextFollow(ctx)
			if ev == nil {
				break
			}
			switch f := ev.Event.(type) {
			case *flow.Flow:
				m.RLock()
				for _, metricObj := range m.registry {
					// Flow will be empty if IP doesnt exist
					metricObj.ProcessFlow(f)
				}
				m.RUnlock()
			default:
				m.l.Warn("Unknown event type", zap.Any("event", ev))
			}
		}

		if err := evReader.Close(); err != nil {
			m.l.Error("Error closing event reader", zap.Error(err))
		}
	}()
}

// updateMetricsContext updates the metrics context by resetting the registry and re-initializing metrics
func (m *Module) updateMetricsContext() {
	// clean old metrics and reset registry
	m.Clear()

	spec := (&api.MetricsSpec{}).WithMetricsContextOptions(requiredMetrics, mm.DefaultCtxOptions(), mm.DefaultCtxOptions())
	ctxType := mm.LocalContext

	exporter.ResetAdvancedMetricsRegistry()

	for _, ctxOption := range spec.ContextOptions {
		switch {
		case strings.Contains(ctxOption.MetricName, metrics.Forward):
			fm := metrics.NewForwardCountMetrics(&ctxOption, m.l, ctxType, true)
			if fm != nil {
				m.registry[ctxOption.MetricName] = fm
			}
		case strings.Contains(ctxOption.MetricName, HNS):
			hns := metrics.NewHNSMetrics(&ctxOption, m.l, ctxType)
			if hns != nil {
				m.registry[ctxOption.MetricName] = hns
			}
		case strings.Contains(ctxOption.MetricName, metrics.Drop):
			dm := metrics.NewDropCountMetrics(&ctxOption, m.l, ctxType, true)
			if dm != nil {
				m.registry[ctxOption.MetricName] = dm
			}
		case strings.Contains(ctxOption.MetricName, metrics.TCP):
			tc := metrics.NewTCPConnectionMetrics(&ctxOption, m.l, ctxType)
			if tc != nil {
				m.registry[ctxOption.MetricName] = tc
			}
			tcp := metrics.NewTCPMetrics(&ctxOption, m.l, ctxType, true)
			if tcp != nil {
				m.registry[ctxOption.MetricName] = tcp
			}
		default:
			m.l.Error("Invalid metric name", zap.String("metricName", ctxOption.MetricName))
		}
	}

	for metricName, metricObj := range m.registry {
		metricObj.Init(metricName)
	}
}

// Clear removes all metrics from the registry
func (m *Module) Clear() {
	for key, metricObj := range m.registry {
		metricObj.Clean()
		delete(m.registry, key)
	}
}
