package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sync"
	"time"

	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/microsoft/retina/test/e2e/common"
	prom_client "github.com/prometheus/client_golang/api"
	prom_v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom_model "github.com/prometheus/common/model"
)

type QueryAndPublish struct {
	Query                       string
	Endpoint                    string
	AdditionalTelemetryProperty map[string]string
	outputFilePath              string
	stop                        chan struct{}
	wg                          sync.WaitGroup
	telemetryClient             *telemetry.TelemetryClient
	appInsightsKey              string
}

func (q *QueryAndPublish) Run() error {
	if q.appInsightsKey != "" {
		telemetry.InitAppInsights(q.appInsightsKey, q.AdditionalTelemetryProperty["retinaVersion"])

		telemetryClient, err := telemetry.NewAppInsightsTelemetryClient("retina-rate-of-growth", q.AdditionalTelemetryProperty)
		if err != nil {
			return fmt.Errorf("error creating telemetry client: %w", err)
		}

		q.telemetryClient = telemetryClient
	}

	q.stop = make(chan struct{})
	q.wg.Add(1)

	go func() {

		t := time.NewTicker(2 * time.Second)

		// First execution
		err := q.getAndPublishMetrics()
		if err != nil {
			log.Fatalf("error getting and publishing metrics: %v", err)
			return
		}

		for {
			select {

			case <-t.C:
				err := q.getAndPublishMetrics()
				if err != nil {
					log.Fatalf("error getting and publishing metrics: %v", err)
					return
				}

			case <-q.stop:
				q.wg.Done()
				return

			}
		}

	}()

	return nil
}

func (q *QueryAndPublish) getAndPublishMetrics() error {
	// ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	// defer cancel()

	client, err := prom_client.NewClient(prom_client.Config{
		Address: q.Endpoint,
	})
	if err != nil {
		return fmt.Errorf("error creating prometheus client: %w", err)
	}

	promApi := prom_v1.NewAPI(client)
	ctx := context.TODO()

	result, warnings, err := promApi.Query(ctx, q.Query, time.Now())
	if err != nil {
		return fmt.Errorf("error querying prometheus: %w", err)
	}
	if len(warnings) > 0 {
		log.Println("query warnings: ", warnings)
	}
	type metrics map[string]string

	allMetrics := []metrics{}

	for _, sample := range result.(prom_model.Vector) {
		instance := string(sample.Metric["instance"])
		samplesScraped := sample.Value.String()

		m := map[string]string{
			"instance":       instance,
			"samplesScraped": samplesScraped,
		}
		allMetrics = append(allMetrics, m)
	}

	// Publish metrics
	if q.telemetryClient != nil {
		log.Println("Publishing metrics to AppInsights")
		for _, metric := range allMetrics {
			q.telemetryClient.TrackEvent("metrics-scraped", metric)

		}
	}

	// Write metrics to file
	if q.outputFilePath != "" {
		log.Println("Writing metrics to file ", q.outputFilePath)

		permissions := 0o644
		file, err := os.OpenFile(q.outputFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fs.FileMode(permissions))
		if err != nil {
			return fmt.Errorf("error writing to csv file: %w", err)
		}
		defer file.Close()

		for _, m := range allMetrics {
			b, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("error marshalling metric: %w", err)
			}
			file.Write(b)
			file.WriteString("\n")
		}

	}

	return nil
}

func (q *QueryAndPublish) Stop() error {
	telemetry.ShutdownAppInsights()
	close(q.stop)
	q.wg.Wait()
	return nil
}

func (q *QueryAndPublish) Prevalidate() error {
	if os.Getenv(common.AzureAppInsightsKeyEnv) == "" {
		log.Println("env ", common.AzureAppInsightsKeyEnv, " not provided")
	}
	q.appInsightsKey = os.Getenv(common.AzureAppInsightsKeyEnv)

	if _, ok := q.AdditionalTelemetryProperty["retinaVersion"]; !ok {
		return fmt.Errorf("retinaVersion is required in AdditionalTelemetryProperty")
	}

	if os.Getenv(common.OutputFilePathEnv) == "" {
		log.Println("Output file path not provided. Metrics will not be written to file")
		return nil
	}
	q.outputFilePath = os.Getenv(common.OutputFilePathEnv)

	log.Println("Output file path provided: ", q.outputFilePath)
	return nil
}
