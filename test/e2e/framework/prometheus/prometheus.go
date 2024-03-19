package prom

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/microsoft/retina/test/retry"
	promclient "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

var (
	ErrNoMetricFound     = fmt.Errorf("no metric found")
	defaultTimeout       = 300 * time.Second
	defaultRetryDelay    = 5 * time.Second
	defaultRetryAttempts = 60
)

func CheckMetric(promAddress string, metricName string, validMetric map[string]string) error {
	defaultRetrier := retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}

	ctx := context.Background()
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()

	metrics := map[string]*promclient.MetricFamily{}
	scrapeMetricsFn := func() error {
		log.Printf("checking for drop metrics on %s", promAddress)
		var err error

		// obtain a full dump of all metrics on the endpoint
		metrics, err = getAllPrometheusMetrics(promAddress)
		if err != nil {
			return fmt.Errorf("could not start port forward within %ds: %w	", defaultTimeout, err)
		}

		// loop through each metric to check for a match,
		// if none is found then log and return an error which will trigger a retry
		err = verifyValidMetricPresent(metricName, metrics, validMetric)
		if err != nil {
			log.Printf("failed to find metric matching %s: %+v\n", metricName, validMetric)
			return ErrNoMetricFound
		}

		return nil
	}

	err := defaultRetrier.Do(pctx, scrapeMetricsFn)
	if err != nil {
		return fmt.Errorf("failed to get prometheus metrics: %w", err)
	}
	return nil
}

func verifyValidMetricPresent(metricName string, data map[string]*promclient.MetricFamily, validMetric map[string]string) error {
	for _, metric := range data {
		if metric.GetName() == metricName {
			for _, metric := range metric.GetMetric() {

				// get all labels and values on the metric
				metricLabels := map[string]string{}
				for _, label := range metric.GetLabel() {
					metricLabels[label.GetName()] = label.GetValue()
				}
				if reflect.DeepEqual(metricLabels, validMetric) {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to find metric matching: %+v: %w", validMetric, ErrNoMetricFound)
}

func getAllPrometheusMetrics(url string) (map[string]*promclient.MetricFamily, error) {
	client := http.Client{}
	resp, err := client.Get(url) //nolint
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %v", resp.Status) //nolint:goerr113,gocritic
	}

	metrics, err := parseReaderPrometheusMetrics(resp.Body)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

func parseReaderPrometheusMetrics(input io.Reader) (map[string]*promclient.MetricFamily, error) {
	var parser expfmt.TextParser
	return parser.TextToMetricFamilies(input) //nolint
}
