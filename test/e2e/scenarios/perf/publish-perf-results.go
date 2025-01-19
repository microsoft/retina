package perf

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/pkg/errors"
)

type PublishPerfResults struct {
	ResultsFile string
}

func (v *PublishPerfResults) Prevalidate() error {
	return nil
}

func (v *PublishPerfResults) Run() error {
	appInsightsKey := os.Getenv(common.AzureAppInsightsKeyEnv)
	if appInsightsKey == "" {
		log.Println("No app insights key provided, skipping publishing results")
		return nil
	}

	resultsFile, err := os.OpenFile(v.ResultsFile, os.O_RDONLY, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open results file")
	}
	defer resultsFile.Close()

	resultBytes, err := io.ReadAll(resultsFile)
	if err != nil {
		return errors.Wrap(err, "failed to read results file")
	}

	var results []RegressionResult
	err = json.Unmarshal(resultBytes, &results)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal results")
	}

	retinaVersion := os.Getenv(generic.DefaultTagEnv)

	// We have checks for them in early steps of the perf test
	// so we can safely assume they are set
	// However, this test will ensure they set if run from a different scope
	if retinaVersion == "" {
		return errors.New(fmt.Sprintf("%s must be set", generic.DefaultTagEnv))
	}

	telemetry.InitAppInsights(appInsightsKey, retinaVersion)
	defer telemetry.ShutdownAppInsights()

	telemetryClient, err := telemetry.NewAppInsightsTelemetryClient("retina-perf-test", map[string]string{
		"retinaVersion": retinaVersion,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create telemetry client")
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
