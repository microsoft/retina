//nolint:revive // package name "common" is used across the E2E test suite
package common

import (
	"errors"
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

var ErrMetricFound = errors.New("unexpected metric found")

type ValidateMetric struct {
	ForwardedPort string
	MetricName    string
	ValidMetrics  []map[string]string
	ExpectMetric  bool
	PartialMatch  bool // If true, only the specified labels need to match (metric can have additional labels)
}

func (v *ValidateMetric) Run() error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.ForwardedPort)

	for _, validMetric := range v.ValidMetrics {
		err := prom.CheckMetric(promAddress, v.MetricName, validMetric, v.PartialMatch)
		if err != nil {
			// If we expect the metric not to be found, return nil if it's not found.
			if !v.ExpectMetric && errors.Is(err, prom.ErrNoMetricFound) {
				log.Printf("metric %s not found, as expected\n", v.MetricName)
				return nil
			}
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}

		// if we expect the metric not to be found, return an error if it is found
		if !v.ExpectMetric {
			return fmt.Errorf("did not expect to find metric %s matching %+v: %w", v.MetricName, validMetric, ErrMetricFound)
		}

		log.Printf("found metric %s matching %+v\n", v.MetricName, validMetric)
	}
	return nil
}

func (v *ValidateMetric) Prevalidate() error {
	return nil
}

func (v *ValidateMetric) Stop() error {
	return nil
}
