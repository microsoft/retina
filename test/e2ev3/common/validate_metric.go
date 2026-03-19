// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package common

import (
	"context"
	"errors"
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
)

var ErrMetricFound = errors.New("unexpected metric found")

// ValidateMetricStep validates Prometheus metrics at a given port.
// Implements flow.Steper via Do(context.Context) error.
type ValidateMetricStep struct {
	ForwardedPort string
	MetricName    string
	ValidMetrics  []map[string]string
	ExpectMetric  bool
	PartialMatch  bool
}

func (v *ValidateMetricStep) Do(_ context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.ForwardedPort)

	for _, validMetric := range v.ValidMetrics {
		err := prom.CheckMetric(promAddress, v.MetricName, validMetric, v.PartialMatch)
		if err != nil {
			if !v.ExpectMetric && errors.Is(err, prom.ErrNoMetricFound) {
				log.Printf("metric %s not found, as expected\n", v.MetricName)
				return nil
			}
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}

		if !v.ExpectMetric {
			return fmt.Errorf("did not expect to find metric %s matching %+v: %w", v.MetricName, validMetric, ErrMetricFound)
		}

		log.Printf("found metric %s matching %+v\n", v.MetricName, validMetric)
	}
	return nil
}
