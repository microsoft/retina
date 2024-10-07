package perf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/microsoft/retina/test/e2e/framework/generic"
)

type PublishPerfResults struct {
	ResultsFile string
}

func (v *PublishPerfResults) Prevalidate() error {
	return nil
}

func (v *PublishPerfResults) Run() error {
	resultsFile, err := os.OpenFile(v.ResultsFile, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open results file: %v", err)
	}
	defer resultsFile.Close()

	resultBytes, err := io.ReadAll(resultsFile)
	if err != nil {
		return fmt.Errorf("failed to read results file: %v", err)
	}

	var results []RegressionResult
	err = json.Unmarshal(resultBytes, &results)
	if err != nil {
		return fmt.Errorf("failed to unmarshal results: %v", err)
	}

	appInsightsKey := os.Getenv("APP_INSIGHTS_KEY")
	retinaVersion := os.Getenv(generic.DefaultTagEnv)

	// We have checks for them in early steps of the perf test
	// so we can safely assume they are set
	// However, this test will ensure they set if run from a different scope
	if appInsightsKey == "" || retinaVersion == "" {
		return fmt.Errorf("APP_INSIGHTS_KEY and %s environment variables must be set", generic.DefaultTagEnv)
	}

	telemetry.InitAppInsights(appInsightsKey, retinaVersion)
	defer telemetry.ShutdownAppInsights()

	telemetryClient, err := telemetry.NewAppInsightsTelemetryClient("retina-perf-test", map[string]string{})
	if err != nil {
		return fmt.Errorf("failed to create telemetry client: %v", err)
	}

	fmt.Printf("Sending telemetry data to app insights\n")
	for _, result := range results {
		err = publishResultMetrics(telemetryClient, result.Label, "benchmark", result.Benchmark)
		if err != nil {
			return fmt.Errorf("failed to publish benchmark metrics: %v", err)
		}
		err = publishResultMetrics(telemetryClient, result.Label, "result", result.Result)
		if err != nil {
			return fmt.Errorf("failed to publish result metrics: %v", err)
		}
		err = publishResultMetrics(telemetryClient, result.Label, "regression", result.Regressions)
		if err != nil {
			return fmt.Errorf("failed to publish regression metrics: %v", err)
		}
	}
	return nil
}

func (v *PublishPerfResults) Stop() error {
	return nil
}

func publishResultMetrics(telemetryClient telemetry.Telemetry, testCase, resultType string, metricMap map[string]float64) error {
	for name, value := range metricMap {
		telemetryClient.TrackMetric(name, value, map[string]string{"resultType": resultType, "testCase": testCase})
	}
	return nil
}
