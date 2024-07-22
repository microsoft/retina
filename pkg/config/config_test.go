// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package config

import (
	"testing"
	"time"
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
		c.EnableAnnotations {
		t.Fatalf("Expeted config should be same as ./testwith/config.yaml; instead got %+v", c)
	}
}
