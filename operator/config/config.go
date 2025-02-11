package config

import (
	"fmt"
	"log"
	"time"

	"github.com/microsoft/retina/pkg/config"
	"github.com/spf13/viper"
)

const MinTelemetryInterval time.Duration = 2 * time.Minute

var (
	DefaultTelemetryInterval       = 5 * time.Minute
	ErrorTelemetryIntervalTooSmall = fmt.Errorf("telemetryInterval smaller than %v is not allowed", MinTelemetryInterval)
)

type OperatorConfig struct {
	config.CaptureConfig `mapstructure:",squash"`

	InstallCRDs     bool   `yaml:"installCRDs"`
	EnableTelemetry bool   `yaml:"enableTelemetry"`
	LogLevel        string `yaml:"logLevel"`
	// EnableRetinaEndpoint indicates whether to enable RetinaEndpoint
	EnableRetinaEndpoint bool          `yaml:"enableRetinaEndpoint"`
	RemoteContext        bool          `yaml:"remoteContext"`
	TelemetryInterval    time.Duration `yaml:"telemetryInterval"`
}

func GetConfig(cfgFileName string) (*OperatorConfig, error) {
	viper.SetConfigType("yaml")
	viper.SetConfigFile(cfgFileName)
	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	viper.AutomaticEnv()

	var cfg OperatorConfig
	viper.SetDefault("EnableRetinaEndpoint", true)
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}

	// If unset, default telemetry interval to 5 minutes.
	if cfg.TelemetryInterval == 0 {
		log.Printf("telemetryInterval is not set, defaulting to %v", DefaultTelemetryInterval)
		cfg.TelemetryInterval = DefaultTelemetryInterval
	} else if cfg.TelemetryInterval < MinTelemetryInterval {
		return nil, ErrorTelemetryIntervalTooSmall
	}

	return &cfg, nil
}
