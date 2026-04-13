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
	"github.com/prometheus/common/model"
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

func formatMetricDetail(name string, mf *promclient.MetricFamily, m *promclient.Metric) string {
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteString("{")
	for i, label := range m.GetLabel() {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%s=%q", label.GetName(), label.GetValue())
	}
	sb.WriteString("}")

	switch mf.GetType() {
	case promclient.MetricType_COUNTER:
		fmt.Fprintf(&sb, " counter:%v", m.GetCounter().GetValue())
	case promclient.MetricType_GAUGE:
		fmt.Fprintf(&sb, " gauge:%v", m.GetGauge().GetValue())
	case promclient.MetricType_HISTOGRAM:
		h := m.GetHistogram()
		fmt.Fprintf(&sb, " histogram:count=%v sum=%v", h.GetSampleCount(), h.GetSampleSum())
	case promclient.MetricType_SUMMARY:
		s := m.GetSummary()
		fmt.Fprintf(&sb, " summary:count=%v sum=%v", s.GetSampleCount(), s.GetSampleSum())
	case promclient.MetricType_UNTYPED:
		fmt.Fprintf(&sb, " untyped:%v", m.GetUntyped().GetValue())
	}

	return sb.String()
}

func verifyValidMetricPresent(metricName string, data map[string]*promclient.MetricFamily, validMetric map[string]string) error {
	for _, mf := range data {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {

				// get all labels and values on the metric
				metricLabels := map[string]string{}
				for _, label := range m.GetLabel() {
					metricLabels[label.GetName()] = label.GetValue()
				}

				// if valid metric is empty, then we just need to make sure the metric and value is present
				if len(validMetric) == 0 && len(metricLabels) > 0 {
					log.Printf("found matching metric: %s", formatMetricDetail(metricName, mf, m))
					return nil
				}

				if reflect.DeepEqual(metricLabels, validMetric) {
					log.Printf("found matching metric: %s", formatMetricDetail(metricName, mf, m))
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
	for _, mf := range data {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {

				// get all labels and values on the metric
				metricLabels := map[string]string{}
				for _, label := range m.GetLabel() {
					metricLabels[label.GetName()] = label.GetValue()
				}

				// if valid metric is empty, then we just need to make sure the metric and value is present
				if len(validMetric) == 0 && len(metricLabels) > 0 {
					log.Printf("found matching metric: %s", formatMetricDetail(metricName, mf, m))
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
					log.Printf("found matching metric: %s", formatMetricDetail(metricName, mf, m))
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to find metric matching: %+v: %w", validMetric, ErrNoMetricFound)
}

func getAllPrometheusMetricsFromBuffer(buf []byte) (map[string]*promclient.MetricFamily, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	reader := strings.NewReader(string(buf))
	return parser.TextToMetricFamilies(reader) //nolint
}

func ParseReaderPrometheusMetrics(input io.Reader) (map[string]*promclient.MetricFamily, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
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
