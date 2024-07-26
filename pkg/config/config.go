// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// Level defines the level of monitor aggregation.
type Level int

const (
	Low Level = iota
	High
)

func (l *Level) UnmarshalText(text []byte) error {
	s := strings.ToLower(string(text))
	switch s {
	case "low":
		*l = Low
	case "high":
		*l = High
	default:
		*l = Low
	}
	return nil
}

func (l *Level) String() string {
	switch *l {
	case Low:
		return "low"
	case High:
		return "high"
	default:
		return "unknown"
	}
}

type Server struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Config struct {
	APIServer                Server        `yaml:"apiServer"`
	LogLevel                 string        `yaml:"logLevel"`
	EnabledPlugin            []string      `yaml:"enabledPlugin"`
	MetricsInterval          time.Duration `yaml:"metricsInterval"`
	EnableTelemetry          bool          `yaml:"enableTelemetry"`
	EnableRetinaEndpoint     bool          `yaml:"enableRetinaEndpoint"`
	EnablePodLevel           bool          `yaml:"enablePodLevel"`
	RemoteContext            bool          `yaml:"remoteContext"`
	EnableAnnotations        bool          `yaml:"enableAnnotations"`
	BypassLookupIPOfInterest bool          `yaml:"bypassLookupIPOfInterest"`
	DataAggregationLevel     Level         `yaml:"dataAggregationLevel"`
}

func GetConfig(cfgFilename string) (*Config, error) {
	if cfgFilename != "" {
		viper.SetConfigFile(cfgFilename)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("/retina/config")
	}

	viper.SetEnvPrefix("retina")
	viper.AutomaticEnv()
	// NOTE(mainred): RetinaEndpoint is currently the only supported solution to cache Pod, and before an alternative is implemented,
	// we make EnableRetinaEndpoint true and cannot be configurable.
	viper.SetDefault("EnableRetinaEndpoint", true)

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("fatal error config file: %s", err)
	}
	var config Config
	decoderConfigOption := func(dc *mapstructure.DecoderConfig) {
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
				if f.Kind() != reflect.String {
					return data, nil
				}
				if t != reflect.TypeOf(Level(0)) {
					return data, nil
				}
				var level Level
				err = level.UnmarshalText([]byte(data.(string)))
				if err != nil {
					return nil, err
				}
				return level, nil
			},
		)
	}
	err = viper.Unmarshal(&config, decoderConfigOption)
	if err != nil {
		return nil, fmt.Errorf("fatal error config file: %s", err)
	}
	// Convert to second.
	config.MetricsInterval = config.MetricsInterval * time.Second

	return &config, nil
}
