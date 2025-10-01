// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package config

import (
	"reflect"
	"testing"
	"time"
)

func TestGetStandaloneConfig(t *testing.T) {
	c, err := GetStandaloneConfig("./testwith/config-standalone.yaml")
	if err != nil {
		t.Fatalf("Expected no error, instead got %+v", err)
	}
	if c.APIServer.Host != "0.0.0.0" ||
		c.APIServer.Port != 10093 ||
		c.LogLevel != "info" ||
		!c.EnableTelemetry ||
		!reflect.DeepEqual(c.EnabledPlugin, []string{"hnsstats"}) ||
		c.DataAggregationLevel != High ||
		c.MetricsInterval != 1*time.Second ||
		c.TelemetryInterval != 15*time.Minute ||
		c.EnrichmentMode != "azure-vnet-statefile" ||
		c.CrictlCommandTimeout != 5*time.Second ||
		c.StateFileLocation != "/generic/file/location/azure-vnet.json" {
		t.Errorf("Expeted config should be same as ./testwith/config-standalone.yaml; instead got %+v", c)
	}
}
