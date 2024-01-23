// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	RetinaNamespace             = "networkobservability"
	retinaControlPlaneNamespace = "controlplane_networkobservability"
)

type CallBackFunc func()

var (
	// CombinedGatherer is the combined registry for all metrics to be exposed by promhttp
	CombinedGatherer *prometheus.Registry
	// AdvancedRegistry is used for advanced metrics. This registry can be reset upon metrics config reconciliation
	AdvancedRegistry     *prometheus.Registry
	DefaultRegistry      *prometheus.Registry
	MetricsServeCallback CallBackFunc
)

func init() {
	DefaultRegistry = prometheus.DefaultRegisterer.(*prometheus.Registry)
	AdvancedRegistry = prometheus.NewRegistry()
	CombinedGatherer = prometheus.NewRegistry()
	CombinedGatherer.MustRegister(AdvancedRegistry)
	CombinedGatherer.MustRegister(DefaultRegistry)
	MetricsServeCallback = func() {}
}

func ResetAdvancedMetricsRegistry() {
	CombinedGatherer.Unregister(AdvancedRegistry)
	AdvancedRegistry = prometheus.NewRegistry()
	CombinedGatherer.MustRegister(AdvancedRegistry)
	MetricsServeCallback()
}

func RegisterMetricsServeCallback(callback CallBackFunc) {
	MetricsServeCallback = callback
}

func CreatePrometheusCounterVecForMetric(r prometheus.Registerer, name, desc string, labels ...string) *prometheus.CounterVec {
	return promauto.With(r).NewCounterVec(
		prometheus.CounterOpts{
			Namespace: RetinaNamespace,
			Name:      name,
			Help:      desc,
		},
		labels,
	)
}

func CreatePrometheusGaugeVecForMetric(r prometheus.Registerer, name, desc string, labels ...string) *prometheus.GaugeVec {
	return promauto.With(r).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: RetinaNamespace,
			Name:      name,
			Help:      desc,
		},
		labels,
	)
}

func CreatePrometheusCounterVecForControlPlaneMetric(r prometheus.Registerer, name, desc string, labels ...string) *prometheus.CounterVec {
	return promauto.With(r).NewCounterVec(
		prometheus.CounterOpts{
			Namespace: retinaControlPlaneNamespace,
			Name:      name,
			Help:      desc,
		},
		labels,
	)
}

func CreatePrometheusHistogramWithLinearBucketsForMetric(r prometheus.Registerer, name, desc string, start, width float64, count int) prometheus.Histogram {
	opts := prometheus.HistogramOpts{
		Namespace: RetinaNamespace,
		Name:      name,
		Help:      desc,
		Buckets:   prometheus.LinearBuckets(start, width, count),
	}
	return promauto.With(r).NewHistogram(opts)
}

func UnregisterMetric(r prometheus.Registerer, metric prometheus.Collector) {
	if metric != nil {
		r.Unregister(metric)
	}
}
