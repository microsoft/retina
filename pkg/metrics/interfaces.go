// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=interfaces.go -destination=mock_types.go -package=metrics

type CounterVec interface {
	WithLabelValues(lvs ...string) prometheus.Counter
	GetMetricWithLabelValues(lvs ...string) (prometheus.Counter, error)
}

type GaugeVec interface {
	WithLabelValues(lvs ...string) prometheus.Gauge
	GetMetricWithLabelValues(lvs ...string) (prometheus.Gauge, error)
}

type Histogram interface {
	Observe(float64)
	// Keep the Write method for testing purposes.
	Write(*dto.Metric) error
}
