// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package config

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type StandaloneConfig struct {
	APIServer            Server        `yaml:"apiServer"`
	LogLevel             string        `yaml:"logLevel"`
	EnableTelemetry      bool          `yaml:"enableTelemetry"`
	EnabledPlugin        []string      `yaml:"enabledPlugin"`
	DataAggregationLevel Level         `yaml:"dataAggregationLevel"`
	MetricsInterval      time.Duration `yaml:"metricsInterval"`
	TelemetryInterval    time.Duration `yaml:"telemetryInterval"`
	EnrichmentMode       string        `yaml:"enrichmentMode"`
	CrictlCommandTimeout time.Duration `yaml:"crictlCommandTimeout"`
	StateFileLocation    string        `yaml:"stateFileLocation"`
}

var (
	DefaultStandaloneConfig = StandaloneConfig{
		LogLevel:             "info",
		EnableTelemetry:      false,
		EnabledPlugin:        []string{"hnsstats"},
		DataAggregationLevel: High,
		MetricsInterval:      time.Second,
		EnrichmentMode:       "crictl",
		TelemetryInterval:    DefaultTelemetryInterval,
		CrictlCommandTimeout: 5 * time.Second,
	}

	ErrMissingStateFileLocation = errors.New("stateFileLocation must be set when using statefile enrichment mode")
)

func GetStandaloneConfig(cfgFilename string) (*StandaloneConfig, error) {
	if cfgFilename != "" {
		viper.SetConfigFile(cfgFilename)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("/retina/config")
	}

	viper.SetEnvPrefix("retina")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("fatal error config file: %w", err)
	}

	var config StandaloneConfig
	decoderConfigOption := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(), // default hook.
		mapstructure.StringToSliceHookFunc(","),     // default hook.
		decodeLevelHook,
	))

	err = viper.Unmarshal(&config, decoderConfigOption)
	if err != nil {
		return nil, fmt.Errorf("fatal error unmarshalling config file: %w", err)
	}

	if config.MetricsInterval == 0 {
		log.Printf("metricsInterval is not set, defaulting to %v", DefaultStandaloneConfig.MetricsInterval)
		config.MetricsInterval = DefaultStandaloneConfig.MetricsInterval
	}

	if config.TelemetryInterval == 0 {
		log.Printf("telemetryInterval is not set, defaulting to %v", DefaultTelemetryInterval)
		config.TelemetryInterval = DefaultTelemetryInterval
	}

	if config.CrictlCommandTimeout == 0 {
		log.Printf("crictlCommandTimeout is not set, defaulting to %v", DefaultStandaloneConfig.CrictlCommandTimeout)
		config.CrictlCommandTimeout = DefaultStandaloneConfig.CrictlCommandTimeout
	}

	switch {
	case config.EnrichmentMode == "crictl":
	case strings.HasSuffix(config.EnrichmentMode, "statefile"):
		if config.StateFileLocation == "" {
			return nil, ErrMissingStateFileLocation
		}
	default:
		log.Printf("invalid enrichmentMode: %s, defaulting to '%s', supported modes: crictl and statefile", config.EnrichmentMode, DefaultStandaloneConfig.EnrichmentMode)
		config.EnrichmentMode = DefaultStandaloneConfig.EnrichmentMode
	}

	return &config, nil
}

func StandaloneConfigAdapter(sc *StandaloneConfig) *Config {
	if sc == nil {
		return nil
	}

	return &Config{
		APIServer:               sc.APIServer,
		LogLevel:                sc.LogLevel,
		EnabledPlugin:           sc.EnabledPlugin,
		MetricsInterval:         sc.MetricsInterval,
		MetricsIntervalDuration: sc.MetricsInterval,
		EnableTelemetry:         sc.EnableTelemetry,
		DataAggregationLevel:    sc.DataAggregationLevel,
		TelemetryInterval:       sc.TelemetryInterval,

		// Fields not applicable to StandaloneConfig, set to default values
		EnableRetinaEndpoint:     false,
		EnablePodLevel:           false,
		EnableConntrackMetrics:   false,
		RemoteContext:            false,
		EnableAnnotations:        false,
		BypassLookupIPOfInterest: false,
		MonitorSockPath:          "",
	}
}
