// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	"context"
	"fmt"
	"maps"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/microsoft/retina/pkg/log"
)

var (
	client  appinsights.TelemetryClient
	version string
	mbShift uint64 = 20

	// property keys
	kernelversion = "kernelversion"
	sysmem        = "sysmem"
	heapalloc     = "heapalloc"
	heapobjects   = "heapobjects"
	heapidle      = "heapidle"
	heapinuse     = "heapinuse"
	heapsys       = "heapsys"
	nextgc        = "nextgc"
	stackinuse    = "stackinuse"
	stacksys      = "stacksys"
	othersysmem   = "othersysmem"
	goroutines    = "goroutines"
)

type Telemetry interface {
	StartPerf(name string) *PerformanceCounter
	StopPerf(counter *PerformanceCounter)
	Heartbeat(ctx context.Context, heartbeatInterval time.Duration)
	TrackEvent(name string, properties map[string]string)
	TrackMetric(name string, value float64, properties map[string]string)
	TrackTrace(name string, severity contracts.SeverityLevel, properties map[string]string)
}

func InitAppInsights(appinsightsId, appVersion string) {
	if client != nil {
		fmt.Printf("appinsights client already initialized")
		return
	}
	telemetryConfig := appinsights.NewTelemetryConfiguration(appinsightsId)
	telemetryConfig.MaxBatchInterval = 1 * time.Second
	client = appinsights.NewTelemetryClientFromConfig(telemetryConfig)

	// Set the app version
	version = appVersion
}

func ShutdownAppInsights() {
	select {
	case <-client.Channel().Close(5 * time.Second): //nolint:gomnd // ignore
		// Five second timeout for retries.

		// If we got here, then all telemetry was submitted
		// successfully, and we can proceed to exiting.
	case <-time.After(30 * time.Second):
		// Thirty second absolute timeout.  This covers any
		// previous telemetry submission that may not have
		// completed before Close was called.

		// There are a number of reasons we could have
		// reached here.  We gave it a go, but telemetry
		// submission failed somewhere.  Perhaps old events
		// were still retrying, or perhaps we're throttled.
		// Either way, we don't want to wait around for it
		// to complete, so let's just exit.
	}
}

type TelemetryClient struct {
	sync.RWMutex
	processName string
	properties  map[string]string
	profile     Perf
}

func NewAppInsightsTelemetryClient(processName string, additionalproperties map[string]string) (*TelemetryClient, error) {
	if client == nil {
		fmt.Println("appinsights client not initialized")
	}

	properties := GetEnvironmentProperties()

	for k, v := range additionalproperties {
		properties[k] = v
	}

	perfProfile, err := NewPerfProfile()
	if err != nil {
		return nil, err
	}

	return &TelemetryClient{
		processName: processName,
		properties:  properties,
		profile:     perfProfile,
	}, nil
}

// TrackPanic function sends the stacktrace and flushes logs only in a goroutine where its call is deferred.
// Panics in other goroutines will not be caught by this recover function.
func TrackPanic() {
	// no telemetry means client is not initialized
	if client == nil {
		return
	}
	if r := recover(); r != nil {
		message := fmt.Sprintf("Panic caused by: %v , Stacktrace %s", r, string(debug.Stack()))
		trace := appinsights.NewTraceTelemetry(message, appinsights.Critical)
		trace.Properties = GetEnvironmentProperties()
		trace.Properties["version"] = version

		// Create trace and track it
		client.Track(trace)

		// Close zapai and flush logs
		if logger := log.Logger(); logger != nil {
			logger.Close()
			time.Sleep(10 * time.Second)
		}

		ShutdownAppInsights()

		// propagate panic so that the pod wil restart.
		panic(r)
	}
}

func GetEnvironmentProperties() map[string]string {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("failed to get hostname with err %v", err)
	}

	return map[string]string{
		"goversion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"numcores":  fmt.Sprintf("%d", runtime.NumCPU()),
		"hostname":  hostname,
		"podname":   os.Getenv(EnvPodName),
	}
}

func (t *TelemetryClient) trackWarning(err error, msg string) {
	t.TrackTrace(msg+": "+err.Error(), contracts.Warning, GetEnvironmentProperties())
}

func (t *TelemetryClient) heartbeat(ctx context.Context) {
	kernelVersion, err := KernelVersion(ctx)
	if err != nil {
		t.trackWarning(err, "failed to get kernel version")
	}

	props := map[string]string{
		kernelversion: kernelVersion,
	}

	cpuProps, err := t.profile.GetCPUUsage()
	if err != nil {
		t.trackWarning(err, "failed to get cpu usage")
	}
	maps.Copy(props, cpuProps)
	maps.Copy(props, t.profile.GetMemoryUsage())
	t.TrackEvent("heartbeat", props)
}

func bToMb(b uint64) uint64 {
	return b >> mbShift
}

func (t *TelemetryClient) TrackEvent(name string, properties map[string]string) {
	event := appinsights.NewEventTelemetry(name)

	if t.properties != nil {
		t.RLock()
		for k, v := range t.properties {
			event.Properties[k] = v
		}

		for k, v := range properties {
			event.Properties[k] = v
		}

		event.Properties["process"] = t.processName
		t.RUnlock()
	}

	client.Track(event)
}

func (t *TelemetryClient) TrackMetric(metricname string, value float64, properties map[string]string) {
	metric := appinsights.NewMetricTelemetry(metricname, value)
	if t.properties != nil {
		t.RLock()
		for k, v := range t.properties {
			metric.Properties[k] = v
		}

		for k, v := range properties {
			metric.Properties[k] = v
		}

		metric.Properties["process"] = t.processName
		t.RUnlock()
	}

	client.Track(metric)
}

func (t *TelemetryClient) TrackTrace(name string, severity contracts.SeverityLevel, properties map[string]string) {
	trace := appinsights.NewTraceTelemetry(name, severity)
	if t.properties != nil {
		t.RLock()
		for k, v := range t.properties {
			trace.Properties[k] = v
		}

		for k, v := range properties {
			trace.Properties[k] = v
		}

		trace.Properties["process"] = t.processName
		t.RUnlock()
	}

	client.Track(trace)
}

func (t *TelemetryClient) TrackException(exception *appinsights.ExceptionTelemetry) {
	if t.properties != nil {
		t.RLock()
		for k, v := range t.properties {
			exception.Properties[k] = v
		}

		exception.Properties["process"] = t.processName

		t.RUnlock()
	}
	client.Track(exception)
}

type PerformanceCounter struct {
	functionName string
	startTime    time.Time
}

// Used with start to measure the execution time of a function
// usage defer telemetry.StopPerf(telemetry.StartPerf("functionName"))
// start perf will be evaluated on defer line, and then stop perf will be evaluated on return
func (t *TelemetryClient) StartPerf(functionName string) *PerformanceCounter {
	return &PerformanceCounter{
		functionName: functionName,
		startTime:    time.Now(),
	}
}

func (t *TelemetryClient) StopPerf(counter *PerformanceCounter) {
	ms := float64(time.Since(counter.startTime).Milliseconds())
	t.TrackMetric(counter.functionName, ms, nil)
}

func (t *TelemetryClient) Heartbeat(ctx context.Context, heartbeatInterval time.Duration) {
	ticker := time.NewTicker(heartbeatInterval) // TODOL: make configurable
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.heartbeat(ctx)
		}
	}
}
