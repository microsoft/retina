// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package config

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	c, err := GetConfig("./testwith/config.yaml")
	if err != nil {
		t.Fatalf("Expected no error, instead got %+v", err)
	}
	if c.APIServer.Host != "0.0.0.0" ||
		c.APIServer.Port != 10093 ||
		c.LogLevel != "info" ||
		c.MetricsInterval != 10*time.Second ||
		len(c.EnabledPlugin) != 3 ||
		c.EnablePodLevel ||
		!c.EnableRetinaEndpoint ||
		c.RemoteContext ||
		c.EnableAnnotations ||
		c.TelemetryInterval != 15*time.Minute ||
		c.DataAggregationLevel != Low ||
		c.DataSamplingRate != 1 ||
		c.PacketParserRingBuffer != PacketParserRingBufferDisabled {
		t.Errorf("Expeted config should be same as ./testwith/config.yaml; instead got %+v", c)
	}
}

func TestGetConfig_SmallTelemetryInterval(t *testing.T) {
	_, err := GetConfig("./testwith/config-small-telemetry-interval.yaml")
	if !errors.Is(err, ErrorTelemetryIntervalTooSmall) {
		t.Errorf("Expected error %s, instead got %s", ErrorTelemetryIntervalTooSmall, err)
	}
}

func TestGetConfig_DefaultTelemetryInterval(t *testing.T) {
	c, err := GetConfig("./testwith/config-without-telemetry-interval.yaml")
	if err != nil {
		t.Errorf("Expected no error, instead got %+v", err)
	}
	if c.TelemetryInterval != DefaultTelemetryInterval {
		t.Errorf("Expected telemetry interval to be %v, instead got %v", DefaultTelemetryInterval, c.TelemetryInterval)
	}
}

func TestDecodeLevelHook(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected interface{}
	}{
		{"low", Low},
		{"high", High},
		{"invalid", Low}, // Unimplemented or invalid input should default to Low
		{123, 123},       // Non-string input should be returned as is
	}

	for _, test := range tests {
		result, err := decodeLevelHook(reflect.TypeOf(test.input), reflect.TypeOf(Level(0)), test.input)
		require.NoError(t, err)
		assert.Equal(t, test.expected, result)
	}
}

func TestDecodePacketParserRingBufferModeHook(t *testing.T) {
	tests := []struct {
		name          string
		input         interface{}
		expected      interface{}
		expectErr     bool
		expectedError error
	}{
		{
			name:     "enabled",
			input:    "enabled",
			expected: PacketParserRingBufferEnabled,
		},
		{
			name:     "disabled",
			input:    "disabled",
			expected: PacketParserRingBufferDisabled,
		},
		{
			name:          "auto not supported",
			input:         "auto",
			expectErr:     true,
			expectedError: ErrPacketParserRingBufferAutoNotSupported,
		},
		{
			name:      "boolean rejected",
			input:     true,
			expectErr: true,
		},
		{
			name:     "non-string passthrough",
			input:    123,
			expected: 123,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := decodePacketParserRingBufferModeHook(
				reflect.TypeOf(test.input),
				reflect.TypeOf(PacketParserRingBufferMode("")),
				test.input,
			)
			if test.expectErr {
				require.Error(t, err)
				if test.expectedError != nil {
					require.ErrorIs(t, err, test.expectedError)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expected, result)
		})
	}
}
