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

	appInsightsKey := os.Getenv("AZURE_APP_INSIGHTS_KEY")
	retinaVersion := os.Getenv(generic.DefaultTagEnv)

	// We have checks for them in early steps of the perf test
	// so we can safely assume they are set
	// However, this test will ensure they set if run from a different scope
	if appInsightsKey == "" || retinaVersion == "" {
		return fmt.Errorf("AZURE_APP_INSIGHTS_KEY and %s environment variables must be set", generic.DefaultTagEnv)
	}

	telemetry.InitAppInsights(appInsightsKey, retinaVersion)
	defer telemetry.ShutdownAppInsights()

	telemetryClient, err := telemetry.NewAppInsightsTelemetryClient("retina-perf-test", map[string]string{
		"retinaVersion": retinaVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to create telemetry client: %v", err)
	}

	fmt.Printf("Sending telemetry data to app insights\n")
	for _, result := range results {
		publishResultEvent(telemetryClient, result.Label, "benchmark", result.Benchmark)
		publishResultEvent(telemetryClient, result.Label, "result", result.Result)
		publishResultEvent(telemetryClient, result.Label, "regression", result.Regressions)
	}
	return nil
}

func (v *PublishPerfResults) Stop() error {
	return nil
}

func publishResultEvent(telemetryClient telemetry.Telemetry, testCase, resultType string, metricMap map[string]float64) {
	event := make(map[string]string)
	for k, v := range metricMap {
		event[k] = fmt.Sprintf("%f", v)
	}
	event["testCase"] = testCase
	event["resultType"] = resultType
	telemetryClient.TrackEvent("retina-perf", event)
}
