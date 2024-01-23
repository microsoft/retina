// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

//go:generate go run github.com/golang/mock/mockgen@v1.6.0 -source=interfaces.go -destination=mock_types.go -package=metrics

type ICounterVec interface {
	WithLabelValues(lvs ...string) prometheus.Counter
	GetMetricWithLabelValues(lvs ...string) (prometheus.Counter, error)
}

type IGaugeVec interface {
	WithLabelValues(lvs ...string) prometheus.Gauge
	GetMetricWithLabelValues(lvs ...string) (prometheus.Gauge, error)
}

type IHistogramVec interface {
	Observe(float64)
	// Keep the Write method for testing purposes.
	Write(*dto.Metric) error
}
