package config_test

import (
	"errors"
	"testing"
	"time"

	"github.com/microsoft/retina/operator/config"
)

func TestGetConfig(t *testing.T) {
	c, err := config.GetConfig("./testwith/config.yaml")
	if err != nil {
		t.Errorf("Expected no error, instead got %+v", err)
	}

	if !c.InstallCRDs ||
		!c.EnableTelemetry ||
		c.LogLevel != "info" ||
		!c.EnableRetinaEndpoint ||
		!c.RemoteContext ||
		c.TelemetryInterval != 15*time.Minute {
		t.Errorf("Expeted config should be same as ./testwith/config.yaml; instead got %+v", c)
	}
}

func TestGetConfig_DefaultTelemetryInterval(t *testing.T) {
	c, err := config.GetConfig("./testwith/config-without-telemetry-interval.yaml")
	if err != nil {
		t.Errorf("Expected no error, instead got %+v", err)
	}

	if c.TelemetryInterval != config.DefaultTelemetryInterval {
		t.Errorf("Expected telemetry interval to be %v, instead got %v", config.DefaultTelemetryInterval, c.TelemetryInterval)
	}
}

func TestGetConfig_SmallTelemetryInterval(t *testing.T) {
	_, err := config.GetConfig("./testwith/config-small-telemetry-interval.yaml")
	if !errors.Is(err, config.ErrorTelemetryIntervalTooSmall) {
		t.Errorf("Expected error %s, instead got %s", config.ErrorTelemetryIntervalTooSmall, err)
	}
}
