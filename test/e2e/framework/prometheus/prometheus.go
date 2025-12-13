package prom

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
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

func CheckMetric(promAddress, metricName string, validMetric map[string]string, partial ...bool) error {
	defaultRetrier := retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}

	ctx := context.Background()
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Default partial to false if not provided
	usePartial := len(partial) > 0 && partial[0]

	metrics := map[string]*promclient.MetricFamily{}
	scrapeMetricsFn := func() error {
		log.Printf("checking for metrics on %s", promAddress)
		var err error

		// obtain a full dump of all metrics on the endpoint
		metrics, err = getAllPrometheusMetricsFromURL(promAddress)
		if err != nil {
			return fmt.Errorf("could not start port forward within %ds: %w	", defaultTimeout, err)
		}

		// loop through each metric to check for a match,
		// if none is found then log and return an error which will trigger a retry
		if usePartial {
			err = verifyValidMetricPresentPartial(metricName, metrics, validMetric)
		} else {
			err = verifyValidMetricPresent(metricName, metrics, validMetric)
		}
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

func CheckMetricFromBuffer(prometheusMetricData []byte, metricName string, validMetric map[string]string) error {
	metrics, err := getAllPrometheusMetricsFromBuffer(prometheusMetricData)
	if err != nil {
		return fmt.Errorf("failed to parse prometheus metrics: %w", err)
	}

	err = verifyValidMetricPresent(metricName, metrics, validMetric)
	if err != nil {
		log.Printf("failed to find metric matching %s: %+v\n", metricName, validMetric)
		return ErrNoMetricFound
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

				// if valid metric is empty, then we just need to make sure the metric and value is present
				if len(validMetric) == 0 && len(metricLabels) > 0 {
					return nil
				}

				if reflect.DeepEqual(metricLabels, validMetric) {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to find metric matching: %+v: %w", validMetric, ErrNoMetricFound)
}

func getAllPrometheusMetricsFromURL(url string) (map[string]*promclient.MetricFamily, error) {
	client := http.Client{}
	resp, err := client.Get(url) //nolint
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %v", resp.Status) //nolint:goerr113,gocritic
	}

	metrics, err := ParseReaderPrometheusMetrics(resp.Body)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// verifyValidMetricPresentPartial checks if a metric exists with labels that contain
// all the key-value pairs in validMetric (partial matching - the metric can have additional labels)
func verifyValidMetricPresentPartial(metricName string, data map[string]*promclient.MetricFamily, validMetric map[string]string) error {
	for _, metric := range data {
		if metric.GetName() == metricName {
			for _, metric := range metric.GetMetric() {

				// get all labels and values on the metric
				metricLabels := map[string]string{}
				for _, label := range metric.GetLabel() {
					metricLabels[label.GetName()] = label.GetValue()
				}

				// if valid metric is empty, then we just need to make sure the metric and value is present
				if len(validMetric) == 0 && len(metricLabels) > 0 {
					return nil
				}

				// Check if all key-value pairs in validMetric exist in metricLabels
				allMatch := true
				for key, value := range validMetric {
					if metricLabels[key] != value {
						allMatch = false
						break
					}
				}

				if allMatch {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to find metric matching: %+v: %w", validMetric, ErrNoMetricFound)
}

func getAllPrometheusMetricsFromBuffer(buf []byte) (map[string]*promclient.MetricFamily, error) {
	var parser expfmt.TextParser
	reader := strings.NewReader(string(buf))
	return parser.TextToMetricFamilies(reader) //nolint
}

func ParseReaderPrometheusMetrics(input io.Reader) (map[string]*promclient.MetricFamily, error) {
	var parser expfmt.TextParser
	return parser.TextToMetricFamilies(input) //nolint
}

// When capturing promethus output via curl and exect, there's a lot
// of garbage at the front
func stripExecGarbage(s string) string {
	index := strings.Index(s, "#")
	if index == -1 {
		// If there's no `#`, return the original string
		return s
	}
	// Slice the string up to the character before the first `#`
	return s[:index]
}
