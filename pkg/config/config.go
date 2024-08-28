// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package config

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
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
		// Default to Low if the text is not recognized.
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
		return ""
	}
}

type Server struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Config describes the complete configuration for a Retina process.
type Config struct {
	APIServer       Server        `yaml:"apiServer"`
	LogLevel        string        `yaml:"logLevel"`
	EnabledPlugin   []string      `yaml:"enabledPlugin"`
	MetricsInterval time.Duration `yaml:"metricsInterval"`
	// Deprecated: Use only MetricsInterval instead in the go code.
	MetricsIntervalDuration  time.Duration `yaml:"metricsIntervalDuration"`
	EnableTelemetry          bool          `yaml:"enableTelemetry"`
	EnableRetinaEndpoint     bool          `yaml:"enableRetinaEndpoint"`
	EnablePodLevel           bool          `yaml:"enablePodLevel"`
	RemoteContext            bool          `yaml:"remoteContext"`
	EnableAnnotations        bool          `yaml:"enableAnnotations"`
	BypassLookupIPOfInterest bool          `yaml:"bypassLookupIPOfInterest"`
	DataAggregationLevel     Level         `yaml:"dataAggregationLevel"`
	MonitorSockPath          string        `yaml:"monitorSockPath"`
}

func GetConfig(files ...string) (*Config, error) {
	if len(files) == 1 {
		cfgFilename := files[0]
		if cfgFilename != "" {
			viper.SetConfigFile(cfgFilename)
		} else {
			viper.SetConfigName("config")
			viper.AddConfigPath("/retina/config")
		}
	} else {
		for _, file := range files {
			viper.SetConfigFile(file)
			if err := viper.MergeInConfig(); err != nil {
				return nil, errors.Wrapf(err, "loading config file %q", file)
			}
		}
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
			mapstructure.StringToTimeDurationHookFunc(), // default hook.
			mapstructure.StringToSliceHookFunc(","),     // default hook.
			decodeLevelHook,
		)
	}
	err = viper.Unmarshal(&config, decoderConfigOption)
	if err != nil {
		return nil, fmt.Errorf("fatal error config file: %s", err)
	}

	if config.MetricsIntervalDuration != 0 {
		config.MetricsInterval = config.MetricsIntervalDuration
	} else if config.MetricsInterval != 0 {
		config.MetricsInterval *= time.Second
		log.Print("metricsInterval is deprecated, please use metricsIntervalDuration instead")
	}

	return &config, nil
}

func decodeLevelHook(field, target reflect.Type, data interface{}) (interface{}, error) {
	// Check if the field we are decoding is a string.
	if field.Kind() != reflect.String {
		return data, nil
	}
	// Check if the type we are decoding to is a Level.
	if target != reflect.TypeOf(Level(0)) {
		return data, nil
	}
	var level Level
	err := level.UnmarshalText([]byte(data.(string)))
	if err != nil {
		return nil, err
	}
	return level, nil
}
