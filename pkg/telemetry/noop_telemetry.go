// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	"context"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

func NewNoopTelemetry() *NoopTelemetry {
	return &NoopTelemetry{}
}

type NoopTelemetry struct{}

func (m NoopTelemetry) StartPerf(functionName string) *PerformanceCounter {
	return &PerformanceCounter{}
}

func (m NoopTelemetry) StopPerf(counter *PerformanceCounter) {
}

func (m NoopTelemetry) Heartbeat(_ context.Context, _ time.Duration, _ ...func() map[string]string) {
}

func (m NoopTelemetry) TrackEvent(name string, properties map[string]string) {
}

func (m NoopTelemetry) TrackMetric(name string, value float64, properties map[string]string) {
}

func (m NoopTelemetry) TrackTrace(name string, severity contracts.SeverityLevel, properties map[string]string) {
}
