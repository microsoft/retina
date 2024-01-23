// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func init() {
	InitAppInsights("test", "test")
}

func TestNewAppInsightsTelemetryClient(t *testing.T) {
	require.NotPanics(t, func() { NewAppInsightsTelemetryClient("test", map[string]string{}) })
}

func TestGetEnvironmentProerties(t *testing.T) {
	properties := GetEnvironmentProperties()

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("failed to get hostname with err %v", err)
	}
	require.NotEmpty(t, properties)
	require.Exactly(
		t,
		map[string]string{
			"goversion": runtime.Version(),
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
			"numcores":  fmt.Sprintf("%d", runtime.NumCPU()),
			"hostname":  hostname,
			"podname":   os.Getenv("POD_NAME"),
		},
		properties,
	)
}

func TestNoopTelemetry(t *testing.T) {
	telemetry := NewNoopTelemetry()
	require.NotNil(t, telemetry)
	require.Equal(t, &NoopTelemetry{}, telemetry)
}

func TestNoopTelemetryStartPerf(t *testing.T) {
	telemetry := NewNoopTelemetry()
	require.NotNil(t, telemetry)
	require.Equal(t, &NoopTelemetry{}, telemetry)

	perf := telemetry.StartPerf("test")
	require.NotNil(t, perf)
	require.Equal(t, &PerformanceCounter{}, perf)
}

func TestHeartbeat(t *testing.T) {
	InitAppInsights("test", "test")
	type fields struct {
		properties map[string]string
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test heartbeat",
			fields: fields{
				properties: map[string]string{
					"test": "test",
				},
			},
			args: args{
				ctx: context.Background(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TelemetryClient{
				RWMutex:    sync.RWMutex{}, //nolint:copylocks
				properties: tt.fields.properties,
			}
			tr.heartbeat(tt.args.ctx)
		})
	}
}

func TestTelemetryClient_StopPerf(t *testing.T) {
	type fields struct {
		properties map[string]string
	}
	type args struct {
		counter *PerformanceCounter
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test stop performance",
			fields: fields{
				properties: map[string]string{
					"test": "test",
				},
			},
			args: args{
				counter: &PerformanceCounter{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TelemetryClient{
				RWMutex:    sync.RWMutex{},
				properties: tt.fields.properties,
			}
			tr.StopPerf(tt.args.counter)
		})
	}
}
